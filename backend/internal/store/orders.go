package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sto/crm/backend/internal/domain"
)

// OrderRepo — доступ к таблице orders.
type OrderRepo struct{ s *Store }

// NewOrderRepo создаёт репозиторий заказов.
func NewOrderRepo(s *Store) *OrderRepo { return &OrderRepo{s: s} }

// Pool возвращает пул соединений для служебных запросов.
func (r *OrderRepo) Pool() *pgxpool.Pool { return r.s.pool }

// orderSelect — базовый SELECT с денормализованными данными клиента и филиала.
const orderSelect = `
	SELECT o.id, o.client_id, o.branch_id, o.car_number, o.service_type,
	       o.preferred_date, o.preferred_time, COALESCE(o.comment, ''), o.status,
	       o.is_client_en_route, o.created_at, o.updated_at,
	       u.name, u.phone, COALESCE(b.address, '')
	FROM orders o
	JOIN users u ON u.id = o.client_id
	LEFT JOIN branches b ON b.id = o.branch_id
`

func scanOrder(row pgx.Row) (*domain.Order, error) {
	var o domain.Order
	var date time.Time
	var clock time.Time
	err := row.Scan(&o.ID, &o.ClientID, &o.BranchID, &o.CarNumber, &o.ServiceType,
		&date, &clock, &o.Comment, &o.Status, &o.IsClientEnRoute,
		&o.CreatedAt, &o.UpdatedAt, &o.ClientName, &o.ClientPhone, &o.BranchAddress)
	if err != nil {
		return nil, err
	}
	o.PreferredDate = date.Format("2006-01-02")
	o.PreferredTime = clock.Format("15:04")
	return &o, nil
}

// OrderFilter — параметры выборки заказов филиала.
type OrderFilter struct {
	Status  domain.OrderStatus
	Date    *time.Time
	Search  string
	OnlyCanceled bool
}

// Create сохраняет новый заказ.
func (r *OrderRepo) Create(ctx context.Context, o *domain.Order) (*domain.Order, error) {
	var id int
	if err := r.s.pool.QueryRow(ctx, `
		INSERT INTO orders (client_id, branch_id, car_number, service_type,
		                    preferred_date, preferred_time, comment, status, is_client_en_route)
		VALUES ($1, $2, $3, $4, $5::date, $6::time, $7, $8, $9)
		RETURNING id`,
		o.ClientID, o.BranchID, o.CarNumber, o.ServiceType,
		o.PreferredDate, o.PreferredTime, o.Comment, o.Status, o.IsClientEnRoute).Scan(&id); err != nil {
		return nil, fmt.Errorf("не удалось создать заказ: %w", err)
	}
	return r.Get(ctx, id)
}

// Get возвращает заказ по идентификатору вместе с данными клиента и филиала.
func (r *OrderRepo) Get(ctx context.Context, id int) (*domain.Order, error) {
	row := r.s.pool.QueryRow(ctx, orderSelect+` WHERE o.id = $1`, id)
	o, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось получить заказ: %w", err)
	}
	return o, nil
}

// ListByClient возвращает заказы клиента (текущие и прошлые).
func (r *OrderRepo) ListByClient(ctx context.Context, clientID int) ([]domain.Order, error) {
	rows, err := r.s.pool.Query(ctx, orderSelect+`
		WHERE o.client_id = $1
		ORDER BY o.preferred_date DESC, o.preferred_time DESC, o.id DESC`, clientID)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить заказы клиента: %w", err)
	}
	defer rows.Close()

	orders := make([]domain.Order, 0, 8)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения заказа: %w", err)
		}
		orders = append(orders, *o)
	}
	return orders, rows.Err()
}

// ListByBranch возвращает заказы филиала с фильтрами по статусу, дате и поиску.
func (r *OrderRepo) ListByBranch(ctx context.Context, branchID int, f OrderFilter) ([]domain.Order, error) {
	var (
		conds = []string{"o.branch_id = $1"}
		args  = []any{branchID}
	)

	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("o.status = $%d", len(args)))
	}
	if f.OnlyCanceled {
		conds = append(conds, "o.status = 'canceled'")
	}
	if f.Date != nil {
		args = append(args, *f.Date)
		conds = append(conds, fmt.Sprintf("o.preferred_date = $%d", len(args)))
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		conds = append(conds, fmt.Sprintf(
			"(lower(u.name) LIKE $%d OR lower(u.phone) LIKE $%d OR lower(o.car_number) LIKE $%d OR lower(o.service_type) LIKE $%d)",
			len(args), len(args), len(args), len(args)))
	}

	query := orderSelect + " WHERE " + strings.Join(conds, " AND ") +
		" ORDER BY o.preferred_date ASC, o.preferred_time ASC, o.id ASC"

	rows, err := r.s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить заказы филиала: %w", err)
	}
	defer rows.Close()

	orders := make([]domain.Order, 0, 16)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения заказа: %w", err)
		}
		orders = append(orders, *o)
	}
	return orders, rows.Err()
}

// ListByBranchAndDate возвращает заказы филиала на указанную дату.
func (r *OrderRepo) ListByBranchAndDate(ctx context.Context, branchID int, date time.Time) ([]domain.Order, error) {
	rows, err := r.s.pool.Query(ctx, orderSelect+`
		WHERE o.branch_id = $1 AND o.preferred_date = $2
		ORDER BY o.preferred_time ASC, o.id ASC`, branchID, date)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить заказы за дату: %w", err)
	}
	defer rows.Close()

	orders := make([]domain.Order, 0, 16)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения заказа: %w", err)
		}
		orders = append(orders, *o)
	}
	return orders, rows.Err()
}

// UpdateStatus меняет статус заказа и возвращает обновлённую запись.
func (r *OrderRepo) UpdateStatus(ctx context.Context, id int, status domain.OrderStatus) (*domain.Order, error) {
	tag, err := r.s.pool.Exec(ctx, `UPDATE orders SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, status, id)
	if err != nil {
		return nil, fmt.Errorf("не удалось обновить статус: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id)
}

// SetEnRoute устанавливает флаг «клиент едет, звонить не нужно».
func (r *OrderRepo) SetEnRoute(ctx context.Context, id int, enRoute bool) (*domain.Order, error) {
	tag, err := r.s.pool.Exec(ctx,
		`UPDATE orders SET is_client_en_route = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, enRoute, id)
	if err != nil {
		return nil, fmt.Errorf("не удалось обновить флаг «скоро буду»: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id)
}

// UpdateSchedule меняет дату и время записи (используется при восстановлении отмены).
func (r *OrderRepo) UpdateSchedule(ctx context.Context, id int, date, clock string) (*domain.Order, error) {
	tag, err := r.s.pool.Exec(ctx,
		`UPDATE orders SET preferred_date = $1::date, preferred_time = $2::time, updated_at = CURRENT_TIMESTAMP WHERE id = $3`,
		date, clock, id)
	if err != nil {
		return nil, fmt.Errorf("не удалось изменить дату записи: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id)
}

// Delete окончательно удаляет заказ, освобождая слот.
func (r *OrderRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.s.pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("не удалось удалить заказ: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountByBranchAndDate возвращает количество активных заказов филиала на дату.
// Отменённые заказы не учитываются: слот считается свободным.
func (r *OrderRepo) CountByBranchAndDate(ctx context.Context, branchID int, date time.Time) (int, error) {
	var cnt int
	err := r.s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM orders
		WHERE branch_id = $1 AND preferred_date = $2 AND status <> 'canceled'`, branchID, date).Scan(&cnt)
	if err != nil {
		return 0, fmt.Errorf("не удалось посчитать заказы: %w", err)
	}
	return cnt, nil
}

// CountsByBranchAndTime — количество заказов на дату, сгруппированных по времени.
func (r *OrderRepo) CountsByBranchAndTime(ctx context.Context, branchID int, date time.Time) (map[string]int, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT to_char(preferred_time, 'HH24:MI') AS slot, COUNT(*)
		FROM orders
		WHERE branch_id = $1 AND preferred_date = $2 AND status <> 'canceled'
		GROUP BY 1`, branchID, date)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить слоты: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var slot string
		var cnt int
		if err := rows.Scan(&slot, &cnt); err != nil {
			return nil, fmt.Errorf("ошибка чтения слотов: %w", err)
		}
		counts[slot] = cnt
	}
	return counts, rows.Err()
}

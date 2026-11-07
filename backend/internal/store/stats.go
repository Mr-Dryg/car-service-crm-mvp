package store

import (
	"context"
	"fmt"
	"time"

	"github.com/sto/crm/backend/internal/domain"
)

// StatsRepo — агрегирующие запросы для календаря и дашборда.
type StatsRepo struct{ s *Store }

// NewStatsRepo создаёт репозиторий статистики.
func NewStatsRepo(s *Store) *StatsRepo { return &StatsRepo{s: s} }

// StatusCounts — количество заказов филиала в разрезе статусов за период.
type StatusCounts struct {
	New        int
	Confirmed  int
	InProgress int
	Ready      int
	Completed  int
	Canceled   int
}

// CountByStatus считает заказы филиала за период [from, to) в разрезе статусов.
// Отменённые заказы учитываются отдельно: они не занимают слот, но важны менеджеру.
func (r *StatsRepo) CountByStatus(ctx context.Context, branchID int, from, to time.Time) (StatusCounts, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT status, COUNT(*)
		FROM orders
		WHERE branch_id = $1 AND preferred_date >= $2 AND preferred_date < $3
		GROUP BY status`, branchID, from, to)
	if err != nil {
		return StatusCounts{}, fmt.Errorf("не удалось получить статистику: %w", err)
	}
	defer rows.Close()

	var c StatusCounts
	for rows.Next() {
		var st domain.OrderStatus
		var cnt int
		if err := rows.Scan(&st, &cnt); err != nil {
			return StatusCounts{}, fmt.Errorf("ошибка чтения статистики: %w", err)
		}
		switch st {
		case domain.StatusNew:
			c.New = cnt
		case domain.StatusConfirmed:
			c.Confirmed = cnt
		case domain.StatusInProgress:
			c.InProgress = cnt
		case domain.StatusReady:
			c.Ready = cnt
		case domain.StatusCompleted:
			c.Completed = cnt
		case domain.StatusCanceled:
			c.Canceled = cnt
		}
	}
	return c, rows.Err()
}

// BranchDayStat — агрегированная статистика по филиалу за конкретный день.
type BranchDayStat struct {
	BranchID   int
	Address    string
	Total      int
	Capacity   int
	ByStatus   map[string]int
	Canceled   int
}

// DayStatsForAllBranches возвращает статистику по всем филиалам за дату.
func (r *StatsRepo) DayStatsForAllBranches(ctx context.Context, date time.Time) ([]BranchDayStat, error) {
	rows, err := r.s.pool.Query(ctx, `
		SELECT b.id, b.address, o.status, COUNT(*)
		FROM branches b
		LEFT JOIN orders o
		       ON o.branch_id = b.id
		      AND o.preferred_date = $1
		GROUP BY b.id, b.address, o.status
		ORDER BY b.id`, date)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить статистику филиалов: %w", err)
	}
	defer rows.Close()

	byBranch := make(map[int]*BranchDayStat)
	order := make([]int, 0, 8)
	for rows.Next() {
		var id int
		var address string
		var st *domain.OrderStatus
		var cnt int
		if err := rows.Scan(&id, &address, &st, &cnt); err != nil {
			return nil, fmt.Errorf("ошибка чтения статистики филиалов: %w", err)
		}
		stt, ok := byBranch[id]
		if !ok {
			stt = &BranchDayStat{BranchID: id, Address: address, ByStatus: map[string]int{}}
			byBranch[id] = stt
			order = append(order, id)
		}
		if st != nil {
			stt.ByStatus[string(*st)] = cnt
			if *st == domain.StatusCanceled {
				stt.Canceled = cnt
			} else {
				stt.Total += cnt
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]BranchDayStat, 0, len(order))
	for _, id := range order {
		result = append(result, *byBranch[id])
	}
	return result, nil
}

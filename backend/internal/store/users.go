package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sto/crm/backend/internal/domain"
)

// ErrNotFound — запись не найдена.
var ErrNotFound = errors.New("не найдено")

// ErrConflict — конфликт уникальности (например, занятый телефон).
var ErrConflict = errors.New("конфликт данных")

// BranchRepo — доступ к таблице branches.
type BranchRepo struct{ s *Store }

// NewBranchRepo создаёт репозиторий филиалов.
func NewBranchRepo(s *Store) *BranchRepo { return &BranchRepo{s: s} }

// List возвращает все филиалы, отсортированные по адресу.
func (r *BranchRepo) List(ctx context.Context) ([]domain.Branch, error) {
	rows, err := r.s.pool.Query(ctx, `SELECT id, address, created_at FROM branches ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить филиалы: %w", err)
	}
	defer rows.Close()

	branches := make([]domain.Branch, 0, 8)
	for rows.Next() {
		var b domain.Branch
		if err := rows.Scan(&b.ID, &b.Address, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("ошибка чтения филиала: %w", err)
		}
		branches = append(branches, b)
	}
	return branches, rows.Err()
}

// Get возвращает филиал по идентификатору.
func (r *BranchRepo) Get(ctx context.Context, id int) (*domain.Branch, error) {
	var b domain.Branch
	err := r.s.pool.QueryRow(ctx, `SELECT id, address, created_at FROM branches WHERE id = $1`, id).
		Scan(&b.ID, &b.Address, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось получить филиал: %w", err)
	}
	return &b, nil
}

// Create добавляет новый филиал.
func (r *BranchRepo) Create(ctx context.Context, address string) (*domain.Branch, error) {
	var b domain.Branch
	err := r.s.pool.QueryRow(ctx,
		`INSERT INTO branches (address) VALUES ($1) RETURNING id, address, created_at`, address).
		Scan(&b.ID, &b.Address, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать филиал: %w", err)
	}
	return &b, nil
}

// UserRepo — доступ к таблице users.
type UserRepo struct{ s *Store }

// NewUserRepo создаёт репозиторий пользователей.
func NewUserRepo(s *Store) *UserRepo { return &UserRepo{s: s} }

// Pool возвращает пул соединений для служебных запросов.
func (r *UserRepo) Pool() *pgxpool.Pool { return r.s.pool }

const userColumns = `id, phone, password_hash, name, role, branch_id, created_at`

// Create добавляет пользователя.
func (r *UserRepo) Create(ctx context.Context, u *domain.User) (*domain.User, error) {
	var created domain.User
	err := r.s.pool.QueryRow(ctx, `
		INSERT INTO users (phone, password_hash, name, role, branch_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+userColumns,
		u.Phone, u.PasswordHash, u.Name, u.Role, u.BranchID).
		Scan(&created.ID, &created.Phone, &created.PasswordHash, &created.Name, &created.Role, &created.BranchID, &created.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("не удалось создать пользователя: %w", err)
	}
	return &created, nil
}

// GetByPhone находит пользователя по номеру телефона.
func (r *UserRepo) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	var u domain.User
	err := r.s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE phone = $1`, phone).
		Scan(&u.ID, &u.Phone, &u.PasswordHash, &u.Name, &u.Role, &u.BranchID, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось найти пользователя: %w", err)
	}
	return &u, nil
}

// Get возвращает пользователя по идентификатору.
func (r *UserRepo) Get(ctx context.Context, id int) (*domain.User, error) {
	var u domain.User
	err := r.s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Phone, &u.PasswordHash, &u.Name, &u.Role, &u.BranchID, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("не удалось найти пользователя: %w", err)
	}
	return &u, nil
}

// Package store реализует слой доступа к данным (PostgreSQL).
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store — фасад над пулом соединений PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New создаёт пул соединений и проверяет подключение.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("неверный DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 10 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать пул: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("не удалось подключиться к БД: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Pool возвращает underlying-пул для редких случаев.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Close закрывает пул соединений.
func (s *Store) Close() { s.pool.Close() }

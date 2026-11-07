package store

import (
	"context"
	"fmt"
)

// migrations — список SQL-скриптов, применяемых при старте приложения.
// Скрипты идемпотентны и применяются в порядке объявления.
var migrations = []string{
	`
DO $$ BEGIN
    CREATE TYPE user_role AS ENUM ('client', 'manager', 'superadmin');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
`,
	`
DO $$ BEGIN
    CREATE TYPE order_status AS ENUM ('new', 'confirmed', 'canceled', 'in_progress', 'ready', 'completed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
`,
	`
CREATE TABLE IF NOT EXISTS branches (
    id SERIAL PRIMARY KEY,
    address VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`,
	`
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    phone VARCHAR(20) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    role user_role NOT NULL DEFAULT 'client',
    branch_id INT REFERENCES branches(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`,
	`
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    client_id INT REFERENCES users(id) ON DELETE CASCADE,
    branch_id INT REFERENCES branches(id) ON DELETE RESTRICT,
    car_number VARCHAR(20) NOT NULL,
    service_type VARCHAR(100) NOT NULL,
    preferred_date DATE NOT NULL,
    preferred_time TIME NOT NULL,
    comment TEXT,
    status order_status NOT NULL DEFAULT 'new',
    is_client_en_route BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`,
	// Триггер автообновления updated_at (в PostgreSQL нет ON UPDATE CURRENT_TIMESTAMP).
	`
CREATE OR REPLACE FUNCTION set_orders_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
`,
	`
DROP TRIGGER IF EXISTS trg_orders_updated_at ON orders;
`,
	`
CREATE TRIGGER trg_orders_updated_at
BEFORE UPDATE ON orders
FOR EACH ROW
EXECUTE FUNCTION set_orders_updated_at();
`,
	// Индексы на часто запрашиваемые поля.
	`CREATE INDEX IF NOT EXISTS idx_users_phone ON users (phone);`,
	`CREATE INDEX IF NOT EXISTS idx_users_branch_role ON users (branch_id, role);`,
	`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);`,
	`CREATE INDEX IF NOT EXISTS idx_orders_preferred_date ON orders (preferred_date);`,
	`CREATE INDEX IF NOT EXISTS idx_orders_branch_date ON orders (branch_id, preferred_date);`,
	`CREATE INDEX IF NOT EXISTS idx_orders_client_id ON orders (client_id);`,
	// Композитный индекс для выборки «заказы филиала за дату со статусом».
	`CREATE INDEX IF NOT EXISTS idx_orders_branch_date_status ON orders (branch_id, preferred_date, status);`,
	// Автоинкремент для переиспользованных branch_id в демо-данных.
	`CREATE SEQUENCE IF NOT EXISTS branches_id_seq OWNED BY branches.id;`,
}

// Migrate применяет миграции: создаёт типы, таблицы и индексы.
func (s *Store) Migrate(ctx context.Context) error {
	for i, stmt := range migrations {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("миграция #%d не выполнена: %w", i+1, err)
		}
	}
	return nil
}

// Package config отвечает за чтение конфигурации приложения из переменных окружения.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config — полная конфигурация сервиса.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret  string
	JWTTTL     time.Duration

	// SlotCapacity — количество параллельных боксов (подъемников) на филиале.
	SlotCapacity int
	// WorkDayStart / WorkDayEnd — границы рабочего дня для таймлайна менеджера.
	WorkDayStart string
	WorkDayEnd   string
	// SeedDemoData — создавать демо-данные (филиалы, менеджер, клиент) при пустой БД.
	SeedDemoData bool
}

// Load читает конфигурацию из окружения, применяя разумные значения по умолчанию.
func Load() (*Config, error) {
	cfg := &Config{
		Port:         env("PORT", "8080"),
		DatabaseURL:  env("DATABASE_URL", "postgres://user:password@localhost:5432/sto_crm?sslmode=disable"),
		JWTSecret:    env("JWT_SECRET", "change-me-in-production-secret-key"),
		JWTTTL:       envDuration("JWT_TTL", 720*time.Hour),
		SlotCapacity: envInt("SLOT_CAPACITY", 4),
		WorkDayStart: env("WORK_DAY_START", "09:00"),
		WorkDayEnd:   env("WORK_DAY_END", "21:00"),
		SeedDemoData: envBool("SEED_DEMO_DATA", true),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET не может быть пустым")
	}
	if cfg.SlotCapacity < 1 {
		return nil, fmt.Errorf("SLOT_CAPACITY должен быть >= 1")
	}
	return cfg, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return def
}

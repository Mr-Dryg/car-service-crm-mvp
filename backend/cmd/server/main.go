// Package main — точка входа веб-платформы CRM для сети автосервисов.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sto/crm/backend/internal/config"
	"github.com/sto/crm/backend/internal/httpapi"
	"github.com/sto/crm/backend/internal/realtime"
	"github.com/sto/crm/backend/internal/service"
	"github.com/sto/crm/backend/internal/store"
)

func main() {
	logger := log.New(os.Stdout, "[sto-crm] ", log.LstdFlags|log.Lmsgprefix)

	if err := run(logger); err != nil {
		logger.Fatalf("фатальная ошибка: %v", err)
	}
}

// run поднимает приложение и корректно завершает его по сигналу.
func run(logger *log.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		return err
	}
	logger.Println("миграции применены")

	// Репозитории.
	branchRepo := store.NewBranchRepo(db)
	userRepo := store.NewUserRepo(db)
	orderRepo := store.NewOrderRepo(db)
	statsRepo := store.NewStatsRepo(db)

	// Сервисы.
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTTTL)
	notifier := service.NewLogNotifier(logger)
	slotSvc := service.NewSlotService(orderRepo, cfg.SlotCapacity, cfg.WorkDayStart, cfg.WorkDayEnd)
	orderSvc := service.NewOrderService(orderRepo, branchRepo, userRepo, authSvc, notifier, slotSvc, cfg.SlotCapacity, cfg.WorkDayStart, cfg.WorkDayEnd)
	adminSvc := service.NewAdminService(branchRepo, userRepo, statsRepo, authSvc, cfg.SlotCapacity)

	// Демо-данные и служебные аккаунты.
	if cfg.SeedDemoData {
		if err := seed(ctx, branchRepo, userRepo, orderRepo, authSvc, cfg, logger); err != nil {
			return err
		}
	}
	if err := service.EnsureSuperadmin(ctx, userRepo, envOr("SUPERADMIN_PHONE", "+79000000001"), envOr("SUPERADMIN_PASSWORD", "admin12345"), "Суперадмин"); err != nil {
		return err
	}

	// HTTP-сервер.
	hub := realtime.NewHub(logger)
	srv := httpapi.New(cfg, authSvc, orderSvc, adminSvc, branchRepo, hub, logger)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0, // без таймаута: WebSocket-соединения долгоживущие
		IdleTimeout:       120 * time.Second,
	}

	// Корректное завершение по Ctrl+C / SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("сервер слушает http://localhost:%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-stop:
		logger.Println("получен сигнал остановки")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return httpServer.Shutdown(shutdownCtx)
}

// envOr читает переменную окружения с запасным значением.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

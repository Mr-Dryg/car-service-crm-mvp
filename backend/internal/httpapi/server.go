package httpapi

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/sto/crm/backend/internal/config"
	"github.com/sto/crm/backend/internal/realtime"
	"github.com/sto/crm/backend/internal/service"
	"github.com/sto/crm/backend/internal/store"
	"github.com/sto/crm/backend/internal/webui"
)

// Server — HTTP-сервер CRM, объединяющий сервисы и realtime-хаб.
type Server struct {
	cfg      *config.Config
	auth     *service.AuthService
	orders   *service.OrderService
	admin    *service.AdminService
	branches *store.BranchRepo
	hub      *realtime.Hub
	logger   *log.Logger
}

// New создаёт HTTP-сервер.
func New(
	cfg *config.Config,
	auth *service.AuthService,
	orders *service.OrderService,
	admin *service.AdminService,
	branches *store.BranchRepo,
	hub *realtime.Hub,
	logger *log.Logger,
) *Server {
	return &Server{cfg: cfg, auth: auth, orders: orders, admin: admin, branches: branches, hub: hub, logger: logger}
}

// Routes строит маршрутизатор приложения.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.loggingMiddleware)

	r.Route("/api", func(api chi.Router) {
		// Публичные эндпоинты.
		api.Get("/branches", s.handleListBranches)
		api.Post("/orders/public", s.handleCreatePublicOrder)
		api.Post("/auth/login", s.handleLogin)

		// Эндпоинты клиента.
		api.Group(func(client chi.Router) {
			client.Use(s.authMiddleware)
			client.Use(s.requireRole("client"))
			client.Get("/client/orders", s.handleClientOrders)
			client.Post("/client/orders/{id}/cancel", s.handleClientCancelOrder)
			client.Post("/client/orders/{id}/en-route", s.handleClientEnRoute)
		})

		// Эндпоинты менеджера.
		api.Group(func(mgr chi.Router) {
			mgr.Use(s.authMiddleware)
			mgr.Use(s.requireRole("manager"))
			mgr.Get("/manager/orders", s.handleManagerOrders)
			mgr.Get("/manager/orders/today", s.handleManagerOrdersToday)
			mgr.Get("/manager/slots", s.handleManagerSlots)
			mgr.Patch("/manager/orders/{id}/status", s.handleManagerChangeStatus)
			mgr.Post("/manager/orders/{id}/restore", s.handleManagerRestoreOrder)
			mgr.Delete("/manager/orders/{id}", s.handleManagerDeleteOrder)
			mgr.Get("/manager/ws", s.handleManagerWS)
		})

		// Эндпоинты суперадминистратора.
		api.Group(func(admin chi.Router) {
			admin.Use(s.authMiddleware)
			admin.Use(s.requireRole("superadmin"))
			admin.Get("/admin/dashboard", s.handleAdminDashboard)
			admin.Post("/admin/branches", s.handleAdminCreateBranch)
			admin.Post("/admin/managers", s.handleAdminCreateManager)
		})
	})

	// Встроенный SPA-фронтенд.
	r.Handle("/*", webui.Handler())

	return r
}

// loggingMiddleware логирует запросы.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if strings.HasPrefix(r.URL.Path, "/api") || r.URL.Path == "/ws" {
			s.logger.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, ww.Status(), time.Since(start).Round(time.Millisecond))
		}
	})
}

// parseID разбирает идентификатор из параметра пути.
func parseID(r *http.Request, name string) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, name))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

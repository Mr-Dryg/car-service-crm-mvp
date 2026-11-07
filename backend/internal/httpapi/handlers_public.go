package httpapi

import (
	"net/http"

	"github.com/sto/crm/backend/internal/realtime"
	"github.com/sto/crm/backend/internal/service"
)

// handleListBranches — GET /api/branches: список филиалов для выпадающего списка.
func (s *Server) handleListBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := s.branches.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, branches)
}

// handleCreatePublicOrder — POST /api/orders/public: экспресс-запись с бесшовной регистрацией.
func (s *Server) handleCreatePublicOrder(w http.ResponseWriter, r *http.Request) {
	var req service.PublicOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	result, err := s.orders.CreatePublicOrder(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}

	// Менеджер филиала мгновенно получает событие о новой заявке.
	s.hub.Publish(realtime.Event{
		Type:     realtime.EventNewOrder,
		BranchID: result.Order.BranchID,
		OrderID:  result.Order.ID,
		Title:    "Новая заявка",
		Message:  result.Order.ClientName + " · " + result.Order.CarNumber + " · " + result.Order.ServiceType,
		Payload:  result.Order,
	})

	writeJSON(w, http.StatusCreated, result)
}

// loginRequest — тело запроса авторизации.
type loginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// handleLogin — POST /api/auth/login: вход по телефону и паролю.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	result, err := s.auth.Login(r.Context(), req.Phone, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleClientOrders — GET /api/client/orders: заказы авторизованного клиента.
func (s *Server) handleClientOrders(w http.ResponseWriter, r *http.Request) {
	claims, _ := ClaimsFromContext(r.Context())
	orders, err := s.orders.ListClientOrders(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

// handleClientCancelOrder — POST /api/client/orders/:id/cancel: отмена заказа клиентом.
func (s *Server) handleClientCancelOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "некорректный идентификатор заказа"})
		return
	}
	claims, _ := ClaimsFromContext(r.Context())

	order, err := s.orders.CancelOrder(r.Context(), id, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}

	// Событие попадает в «Красную зону» менеджера филиала.
	s.hub.Publish(realtime.Event{
		Type:     realtime.EventOrderCanceled,
		BranchID: order.BranchID,
		OrderID:  order.ID,
		Title:    "Запись отменена клиентом",
		Message:  order.ClientName + " · " + order.CarNumber + " · " + order.PreferredDate + " " + order.PreferredTime,
		Payload:  order,
	})

	writeJSON(w, http.StatusOK, order)
}

// handleClientEnRoute — POST /api/client/orders/:id/en-route: «Подтверждаю, скоро буду».
func (s *Server) handleClientEnRoute(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "некорректный идентификатор заказа"})
		return
	}
	claims, _ := ClaimsFromContext(r.Context())

	order, err := s.orders.SetEnRoute(r.Context(), id, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}

	// Менеджер мгновенно видит зелёный статус «клиент едет, звонить не нужно».
	s.hub.Publish(realtime.Event{
		Type:     realtime.EventClientEnRoute,
		BranchID: order.BranchID,
		OrderID:  order.ID,
		Title:    "Клиент едет",
		Message:  order.ClientName + " · " + order.CarNumber + " — звонить не нужно",
		Payload:  order,
	})

	writeJSON(w, http.StatusOK, order)
}

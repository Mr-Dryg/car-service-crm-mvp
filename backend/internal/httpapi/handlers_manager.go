package httpapi

import (
	"net/http"
	"time"

	"github.com/sto/crm/backend/internal/domain"
	"github.com/sto/crm/backend/internal/realtime"
	"github.com/sto/crm/backend/internal/service"
)

// handleManagerOrders — GET /api/manager/orders: заказы филиала с фильтрами.
func (s *Server) handleManagerOrders(w http.ResponseWriter, r *http.Request) {
	claims, _ := ClaimsFromContext(r.Context())
	if claims.BranchID == 0 {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "менеджер не привязан к филиалу"})
		return
	}

	q := r.URL.Query()
	filter := service.ManagerListFilter{
		Status:       q.Get("status"),
		Date:         q.Get("date"),
		Search:       q.Get("search"),
		OnlyCanceled: q.Get("canceled") == "1" || q.Get("canceled") == "true",
	}

	orders, err := s.orders.ListBranchOrders(r.Context(), claims.BranchID, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

// handleManagerOrdersToday — GET /api/manager/orders/today: план на сегодня.
func (s *Server) handleManagerOrdersToday(w http.ResponseWriter, r *http.Request) {
	claims, _ := ClaimsFromContext(r.Context())
	if claims.BranchID == 0 {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "менеджер не привязан к филиалу"})
		return
	}

	orders, err := s.orders.ListToday(r.Context(), claims.BranchID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

// handleManagerSlots — GET /api/manager/slots?date=YYYY-MM-DD: интеллектуальный календарь.
func (s *Server) handleManagerSlots(w http.ResponseWriter, r *http.Request) {
	claims, _ := ClaimsFromContext(r.Context())
	if claims.BranchID == 0 {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "менеджер не привязан к филиалу"})
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		now := time.Now()
		date = now.Format("2006-01-02")
	}

	slots, err := s.orders.GetSlots(r.Context(), claims.BranchID, date)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, slots)
}

// changeStatusRequest — тело запроса смены статуса.
type changeStatusRequest struct {
	Status string `json:"status"`
}

// handleManagerChangeStatus — PATCH /api/manager/orders/:id/status: смена статуса заказа.
func (s *Server) handleManagerChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "некорректный идентификатор заказа"})
		return
	}
	claims, _ := ClaimsFromContext(r.Context())

	var req changeStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	order, err := s.orders.ChangeStatus(r.Context(), id, claims.BranchID, domain.OrderStatus(req.Status))
	if err != nil {
		writeError(w, err)
		return
	}

	s.hub.Publish(realtime.Event{
		Type:     realtime.EventOrderUpdated,
		BranchID: order.BranchID,
		OrderID:  order.ID,
		Title:    "Статус изменён",
		Message:  order.ClientName + " · " + order.Status.Label(),
		Payload:  order,
	})

	writeJSON(w, http.StatusOK, order)
}

// restoreOrderRequest — тело запроса восстановления отменённого заказа.
type restoreOrderRequest struct {
	PreferredDate string `json:"preferred_date"`
	PreferredTime string `json:"preferred_time"`
}

// handleManagerRestoreOrder — POST /api/manager/orders/:id/restore: восстановление с возможностью смены даты.
func (s *Server) handleManagerRestoreOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "некорректный идентификатор заказа"})
		return
	}
	claims, _ := ClaimsFromContext(r.Context())

	var req restoreOrderRequest
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, err)
			return
		}
	}

	order, err := s.orders.RestoreOrder(r.Context(), id, claims.BranchID, req.PreferredDate, req.PreferredTime)
	if err != nil {
		writeError(w, err)
		return
	}

	s.hub.Publish(realtime.Event{
		Type:     realtime.EventOrderUpdated,
		BranchID: order.BranchID,
		OrderID:  order.ID,
		Title:    "Запись восстановлена",
		Message:  order.ClientName + " · " + order.PreferredDate + " " + order.PreferredTime,
		Payload:  order,
	})

	writeJSON(w, http.StatusOK, order)
}

// handleManagerDeleteOrder — DELETE /api/manager/orders/:id: окончательное удаление отменённого заказа.
func (s *Server) handleManagerDeleteOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r, "id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "некорректный идентификатор заказа"})
		return
	}
	claims, _ := ClaimsFromContext(r.Context())

	order, err := s.orders.GetOrder(r.Context(), id, claims.BranchID)
	if err != nil {
		writeError(w, err)
		return
	}

	if err := s.orders.DeleteOrder(r.Context(), id, claims.BranchID); err != nil {
		writeError(w, err)
		return
	}

	s.hub.Publish(realtime.Event{
		Type:     realtime.EventOrderDeleted,
		BranchID: order.BranchID,
		OrderID:  order.ID,
		Title:    "Отмена обработана",
		Message:  order.ClientName + " · запись удалена, слот освобождён",
		Payload:  map[string]any{"id": order.ID},
	})

	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": order.ID})
}

// handleManagerWS — GET /api/manager/ws: WebSocket-канал событий для менеджера.
func (s *Server) handleManagerWS(w http.ResponseWriter, r *http.Request) {
	claims, _ := ClaimsFromContext(r.Context())
	if claims.BranchID == 0 {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "менеджер не привязан к филиалу"})
		return
	}
	s.hub.ServeWS(w, r, claims.BranchID)
}

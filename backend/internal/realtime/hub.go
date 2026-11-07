// Package realtime реализует доставку событий менеджеру в реальном времени через WebSocket.
package realtime

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Типы событий для менеджера.
const (
	EventNewOrder       = "order.new"
	EventOrderCanceled = "order.canceled"
	EventOrderUpdated  = "order.updated"
	EventClientEnRoute = "order.en_route"
	EventOrderDeleted  = "order.deleted"
)

// Event — событие, отправляемое менеджеру.
type Event struct {
	Type    string `json:"type"`
	BranchID int   `json:"branch_id,omitempty"`
	OrderID int    `json:"order_id,omitempty"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Payload any    `json:"payload,omitempty"`
}

// Hub — центр управления WebSocket-соединениями менеджеров.
type Hub struct {
	mu       sync.RWMutex
	clients  map[*client]struct{}
	upgrader websocket.Upgrader
	logger   *log.Logger
}

type client struct {
	conn     *websocket.Conn
	branchID int
	send     chan []byte
	hub      *Hub
	closeOne sync.Once
}

// NewHub создаёт хаб realtime-событий.
func NewHub(logger *log.Logger) *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		logger: logger,
	}
}

// Publish рассылает событие всем менеджерам филиала.
// Если branchID == 0, событие получат все подключённые менеджеры.
func (h *Hub) Publish(evt Event) {
	payload, err := json.Marshal(evt)
	if err != nil {
		h.logger.Printf("[WS] ошибка сериализации события: %v", err)
		return
	}

	h.mu.RLock()
	targets := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		if evt.BranchID == 0 || c.branchID == evt.BranchID {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		select {
		case c.send <- payload:
		default:
			// Буфер переполнен — клиент не успевает читать, закрываем соединение.
			h.remove(c)
		}
	}
}

// remove отключает клиента от хаба.
func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// ServeWS обрабатывает WebSocket-подключение менеджера.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, branchID int) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("[WS] ошибка апгрейда соединения: %v", err)
		return
	}

	c := &client{conn: conn, branchID: branchID, send: make(chan []byte, 32), hub: h}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	h.logger.Printf("[WS] менеджер филиала %d подключён (всего %d)", branchID, h.Count())

	go c.writePump()
	c.readPump()
}

// Count возвращает количество активных подключений.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// readPump читает сообщения клиента (пинги) и обрабатывает закрытие соединения.
func (c *client) readPump() {
	defer func() {
		c.hub.remove(c)
		c.closeOne.Do(func() { c.conn.Close() })
		c.hub.logger.Printf("[WS] менеджер филиала %d отключён (осталось %d)", c.branchID, c.hub.Count())
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump отправляет события клиенту и поддерживает ping-сообщения.
func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.hub.remove(c)
		c.closeOne.Do(func() { c.conn.Close() })
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Package httpapi содержит HTTP-роутинг, middleware и обработчики запросов.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/sto/crm/backend/internal/domain"
	"github.com/sto/crm/backend/internal/service"
)

// ctxKey — тип для ключей контекста.
type ctxKey string

const claimsKey ctxKey = "claims"

// ClaimsFromContext извлекает данные авторизации из контекста.
func ClaimsFromContext(ctx context.Context) (*service.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*service.Claims)
	return c, ok
}

// errorResponse — единый формат ошибки API.
type errorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
}

// writeJSON отправляет JSON-ответ.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("[HTTP] ошибка кодирования ответа: %v", err)
	}
}

// writeError отправляет ошибку в едином формате, транслируя код сервиса в HTTP-статус.
func writeError(w http.ResponseWriter, err error) {
	var verr *service.ValidationError
	switch {
	case errors.As(err, &verr):
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: verr.Message, Fields: verr.Fields})
	case errors.Is(err, service.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "неверный телефон или пароль"})
	case errors.Is(err, service.ErrForbidden):
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "доступ запрещён"})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "объект не найден"})
	case errors.Is(err, service.ErrConflict):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "конфликт данных"})
	default:
		log.Printf("[HTTP] внутренняя ошибка: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "внутренняя ошибка сервера"})
	}
}

// decodeJSON читает и разбирает JSON-тело запроса.
func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return &service.ValidationError{Message: "Некорректный формат запроса"}
	}
	return nil
}

// bearerToken извлекает токен из заголовка Authorization, cookie или query-параметра.
// Query-параметр нужен для WebSocket: браузер не позволяет задавать заголовки
// при установке WS-соединения, поэтому токен передаётся в адресе.
func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(strings.ToLower(h), "bearer ") {
			return strings.TrimSpace(h[7:])
		}
	}
	if c, err := r.Cookie("token"); err == nil {
		return c.Value
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return strings.TrimSpace(t)
	}
	return ""
}

// authMiddleware проверяет JWT и кладёт claims в контекст.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "требуется авторизация"})
			return
		}
		claims, err := s.auth.ParseToken(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "недействительный или истёкший токен"})
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRole ограничивает доступ по роли пользователя.
func (s *Server) requireRole(role domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "требуется авторизация"})
				return
			}
			if claims.Role != role {
				writeJSON(w, http.StatusForbidden, errorResponse{Error: "недостаточно прав"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

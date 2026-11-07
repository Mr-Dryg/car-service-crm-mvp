package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/sto/crm/backend/internal/service"
)

// TestDecodeJSON проверяет разбор тела запроса.
func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Status string `json:"status"`
	}

	req := httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(`{"status":"confirmed"}`))
	var p payload
	if err := decodeJSON(req, &p); err != nil {
		t.Fatalf("ожидалась ошибка nil, получено %v", err)
	}
	if p.Status != "confirmed" {
		t.Errorf("status = %q, ожидалось confirmed", p.Status)
	}

	// Неизвестное поле должно отклоняться.
	req = httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(`{"unknown":1}`))
	if err := decodeJSON(req, &p); err == nil {
		t.Error("ожидалась ошибка для неизвестного поля")
	}

	// Некорректный JSON.
	req = httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(`{`))
	if err := decodeJSON(req, &p); err == nil {
		t.Error("ожидалась ошибка для некорректного JSON")
	}
}

// TestBearerToken проверяет извлечение токена из разных источников.
func TestBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	if got := bearerToken(req); got != "header-token" {
		t.Errorf("токен из заголовка = %q, ожидалось header-token", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "cookie-token"})
	if got := bearerToken(req); got != "cookie-token" {
		t.Errorf("токен из cookie = %q, ожидалось cookie-token", got)
	}

	// WebSocket не позволяет передать заголовок — токен идёт в query.
	req = httptest.NewRequest(http.MethodGet, "/api/manager/ws?token=query-token", nil)
	if got := bearerToken(req); got != "query-token" {
		t.Errorf("токен из query = %q, ожидалось query-token", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	if got := bearerToken(req); got != "" {
		t.Errorf("токен без источника = %q, ожидалась пустая строка", got)
	}
}

// TestWriteErrorStatusCodes проверяет трансляцию ошибок в HTTP-статусы.
func TestWriteErrorStatusCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"validation", &service.ValidationError{Message: "ошибка"}, http.StatusUnprocessableEntity},
		{"credentials", service.ErrInvalidCredentials, http.StatusUnauthorized},
		{"forbidden", service.ErrForbidden, http.StatusForbidden},
		{"not found", service.ErrNotFound, http.StatusNotFound},
		{"conflict", service.ErrConflict, http.StatusConflict},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeError(rec, c.err)
			if rec.Code != c.want {
				t.Errorf("код = %d, ожидалось %d", rec.Code, c.want)
			}

			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("не удалось разобрать ответ: %v", err)
			}
			if body.Error == "" {
				t.Error("ожидалось непустое сообщение об ошибке")
			}
		})
	}
}

// TestParseID проверяет разбор идентификатора из пути.
func TestParseID(t *testing.T) {
	newReq := func(param, value string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add(param, value)
		return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}

	id, ok := parseID(newReq("id", "42"), "id")
	if !ok || id != 42 {
		t.Errorf("parseID = (%d, %v), ожидалось (42, true)", id, ok)
	}

	if _, ok := parseID(newReq("id", "abc"), "id"); ok {
		t.Error("ожидалось false для нечислового идентификатора")
	}
	if _, ok := parseID(newReq("id", "0"), "id"); ok {
		t.Error("ожидалось false для нулевого идентификатора")
	}
}

// TestWriteJSON проверяет формирование JSON-ответа.
func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"id": "1"})

	if rec.Code != http.StatusCreated {
		t.Errorf("код = %d, ожидалось %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("не удалось разобрать ответ: %v", err)
	}
	if out["id"] != "1" {
		t.Errorf("id = %q, ожидалось 1", out["id"])
	}
}

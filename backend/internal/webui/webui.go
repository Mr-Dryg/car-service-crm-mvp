// Package webui встраивает собранный SPA-фронтенд в бинарь Go-сервера.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// Handler возвращает HTTP-обработчик, отдающий SPA-фронтенд.
// Неизвестные пути отдаются index.html, чтобы работал history-routing.
func Handler() http.Handler {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	files := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "." || clean == "/" {
			serveIndex(w, r, sub)
			return
		}

		if _, err := fs.Stat(sub, clean); err != nil {
			// Маршрут SPA (например /manager) — отдаём index.html.
			serveIndex(w, r, sub)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// serveIndex отдаёт index.html с корректным Content-Type.
func serveIndex(w http.ResponseWriter, r *http.Request, sub fs.FS) {
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

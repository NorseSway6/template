package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

const unknownRoute = "unknown"

// routePattern возвращает шаблон маршрута.
func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.Routes == nil {
		return unknownRoute
	}
	tctx := chi.NewRouteContext()
	if rctx.Routes.Match(tctx, r.Method, r.URL.Path) {
		if p := tctx.RoutePattern(); p != "" {
			return p
		}
	}
	return unknownRoute
}

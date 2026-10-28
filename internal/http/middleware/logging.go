package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Logging представляет middleware для логирования HTTP-запросов.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := newStatusWriter(w)

			state := recoveryStateFromContext(r.Context())
			if state != nil {
				state.setSpanContext(trace.SpanContextFromContext(r.Context()))
			}
			defer func() {
				if rec := recover(); rec != nil {
					if state != nil && rec != http.ErrAbortHandler {
						state.setCallback(func(status int) {
							logRequest(log, r, status, time.Since(start))
						})
					}
					panic(rec)
				}
				logRequest(log, r, sw.status, time.Since(start))
			}()

			next.ServeHTTP(sw, r)
		})
	}
}

// logRequest записывает информацию о запросе в лог.
func logRequest(log *slog.Logger, r *http.Request, status int, duration time.Duration) {
	level := slog.LevelInfo
	if status >= http.StatusInternalServerError {
		level = slog.LevelError
	}
	log.Log(r.Context(), level, "request handled",
		"method", r.Method,
		"route", routePattern(r),
		"status", status,
		"duration_ms", duration.Milliseconds(),
	)
}

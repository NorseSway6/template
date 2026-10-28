package middleware

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics представляет middleware для сбора метрик HTTP-запросов.
type Metrics struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// NewMetrics создает новый экземпляр Metrics с инициализацией необходимых метрик.
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter("github.com/NorseSway6/template.git/internal/http/middleware")

	duration, err := meter.Float64Histogram("http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create duration histogram: %w", err)
	}
	active, err := meter.Int64UpDownCounter("http.server.active_requests",
		metric.WithUnit("{request}"),
		metric.WithDescription("Number of in-flight HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active requests counter: %w", err)
	}
	return &Metrics{duration: duration, active: active}, nil
}

// Middleware возвращает middleware для сбора метрик HTTP-запросов.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		method := attribute.String("http.request.method", r.Method)
		route := attribute.String("http.route", routePattern(r))

		activeAttrs := metric.WithAttributes(method, route)
		m.active.Add(ctx, 1, activeAttrs)
		defer m.active.Add(ctx, -1, activeAttrs)

		sw := newStatusWriter(w)
		start := time.Now()

		next.ServeHTTP(sw, r)

		m.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
			method, route,
			attribute.Int("http.response.status_code", sw.status),
		))
	})
}

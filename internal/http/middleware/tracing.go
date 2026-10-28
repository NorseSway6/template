package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

// Tracing возвращает middleware, который создает и настраивает спаны для каждого HTTP-запроса.
func Tracing(next http.Handler) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		route := routePattern(r)
		span := trace.SpanFromContext(r.Context())
		span.SetName(r.Method + " " + route)
		span.SetAttributes(attribute.String("http.route", route))
	})

	return otelhttp.NewHandler(inner, "http.server",
		otelhttp.WithMeterProvider(noop.NewMeterProvider()),
	)
}

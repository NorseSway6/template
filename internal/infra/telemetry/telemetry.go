// Package telemetry configures OpenTelemetry once per service process.
package telemetry

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Setup installs process-wide providers and propagation, and returns a context-aware
// logger. The caller must drain HTTP handlers before calling shutdown with a fresh,
// bounded context. OTEL_SERVICE_NAME overrides defaultName; other settings use OTEL_*.
func Setup(ctx context.Context, defaultName string) (_ *slog.Logger, shutdown func(context.Context) error, err error) {
	var closers []func(context.Context) error
	level, levelErr := ParseLevel(os.Getenv("LOG_LEVEL"))
	if levelErr != nil {
		level = slog.LevelInfo
	}
	service := os.Getenv("OTEL_SERVICE_NAME")
	if service == "" {
		service = defaultName
	}
	logger := NewJSONLogger(level, service, os.Stdout)

	shutdown = func(ctx context.Context) error {
		var result error
		for i := len(closers) - 1; i >= 0; i-- {
			result = errors.Join(result, closers[i](ctx))
		}
		return result
	}
	defer func() {
		if err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = errors.Join(err, shutdown(cleanupCtx))
		}
	}()
	if levelErr != nil {
		return logger, shutdown, levelErr
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return logger, shutdown, errors.New("set OTEL_EXPORTER_OTLP_ENDPOINT: load the generated .env first")
	}
	if protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); protocol != "" && protocol != "http/protobuf" {
		return logger, shutdown, errors.New("this example uses OTLP/HTTP protobuf exporters, not gRPC")
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(defaultName)),
		resource.WithFromEnv(), // Explicit per-process OTEL_SERVICE_NAME wins.
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceInstanceID(rand.Text())),
	)
	if err != nil {
		return logger, shutdown, err
	}
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithTimeout(2*time.Second))
	if err != nil {
		return logger, shutdown, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(time.Second)),
	)
	closers = append(closers, tp.Shutdown)

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithTimeout(2*time.Second))
	if err != nil {
		return logger, shutdown, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		// OTEL_METRIC_EXPORT_INTERVAL is read by the SDK, in milliseconds.
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)
	closers = append(closers, mp.Shutdown)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	// Context travels with the business HTTP request, not over OTLP.
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return logger, shutdown, nil
}

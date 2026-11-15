package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRecoverRecordsPanicAcrossMiddleware(t *testing.T) {
	spanExporter := tracetest.NewInMemoryExporter()
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(spanExporter)),
	)
	t.Cleanup(func() {
		if err := traceProvider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown trace provider: %v", err)
		}
	})
	previousTraceProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(traceProvider)
	t.Cleanup(func() { otel.SetTracerProvider(previousTraceProvider) })

	metricReader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))
	t.Cleanup(func() {
		if err := meterProvider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown meter provider: %v", err)
		}
	})
	previousMeterProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(meterProvider)
	t.Cleanup(func() { otel.SetMeterProvider(previousMeterProvider) })

	metrics, err := NewMetrics()
	if err != nil {
		t.Fatalf("create HTTP metrics: %v", err)
	}

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	router := chi.NewRouter()
	router.Use(
		Recover(logger),
		Tracing,
		Logging(logger),
		metrics.Middleware,
	)
	router.Get("/trips/{tripId}", func(http.ResponseWriter, *http.Request) {
		panic("handler failed")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/trips/test-id", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusInternalServerError)
	}

	logs := logOutput.String()
	if !strings.Contains(logs, `"msg":"request handled"`) || !strings.Contains(logs, `"status":500`) {
		t.Errorf("request log does not report status 500: %s", logs)
	}

	spans := spanExporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
	assertAttribute(t, spans[0].Attributes, "http.route", "/trips/{tripId}")
	assertAttribute(t, spans[0].Attributes, "http.response.status_code", int64(http.StatusInternalServerError))

	var exported metricdata.ResourceMetrics
	if err := metricReader.Collect(context.Background(), &exported); err != nil {
		t.Fatalf("collect HTTP metrics: %v", err)
	}
	for _, scopeMetrics := range exported.ScopeMetrics {
		for _, metric := range scopeMetrics.Metrics {
			if metric.Name != "http.server.request.duration" {
				continue
			}
			histogram, ok := metric.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("duration metric has data type %T, want Histogram[float64]", metric.Data)
			}
			if len(histogram.DataPoints) != 1 {
				t.Fatalf("duration metric has %d data points, want 1", len(histogram.DataPoints))
			}
			point := histogram.DataPoints[0]
			assertSetAttribute(t, point.Attributes, "http.response.status_code", int64(http.StatusInternalServerError))
			assertSetAttribute(t, point.Attributes, "http.route", "/trips/{tripId}")
			if point.Count != 1 {
				t.Errorf("duration histogram count = %d, want 1", point.Count)
			}
			return
		}
	}
	t.Fatal("exported HTTP request duration histogram was not found")
}

func assertAttribute(t *testing.T, attributes []attribute.KeyValue, key string, want any) {
	t.Helper()
	for _, item := range attributes {
		if string(item.Key) == key {
			if got := item.Value.AsInterface(); got != want {
				t.Errorf("%s = %v, want %v", key, got, want)
			}
			return
		}
	}
	t.Errorf("attribute %q not found", key)
}

func assertSetAttribute(t *testing.T, attributes attribute.Set, key string, want any) {
	t.Helper()
	got, ok := attributes.Value(attribute.Key(key))
	if !ok || got.AsInterface() != want {
		t.Errorf("%s = %v (present %t), want %v", key, got.AsInterface(), ok, want)
	}
}

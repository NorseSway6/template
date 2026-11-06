package domainerror

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/telemetry"
)

func TestProblem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		wantCode       string
		wantCodeStatus int
	}{
		{name: "validation", err: &trip.ValidationError{Field: "body", Reason: "invalid JSON"}, wantCode: "invalid_request", wantCodeStatus: http.StatusBadRequest},
		{name: "trip not found", err: trip.ErrTripNotFound, wantCode: "trip_not_found", wantCodeStatus: http.StatusNotFound},
		{name: "trip completed", err: trip.ErrTripCompleted, wantCode: "trip_completed", wantCodeStatus: http.StatusConflict},
		{name: "driver busy", err: trip.ErrDriverBusy, wantCode: "driver_busy", wantCodeStatus: http.StatusConflict},
		{name: "default", err: assertErr("boom"), wantCode: "internal_error", wantCodeStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, body := Problem(tt.err)
			if status != tt.wantCodeStatus {
				t.Fatalf("status = %d, want %d", status, tt.wantCodeStatus)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", body.Code, tt.wantCode)
			}
		})
	}
}

func TestWriteErrorLogsInternalCauseWithTraceContext(t *testing.T) {
	var logs bytes.Buffer
	logger := telemetry.NewJSONLogger(0, "test", &logs)
	traceID := trace.TraceID{1}
	spanID := trace.SpanID{2}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)
	ctx = WithLogger(ctx, logger)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/123", nil).WithContext(ctx)
	res := httptest.NewRecorder()
	sourceErr := errors.New("database connection failed")

	WriteError(res, req, sourceErr)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
	}
	if strings.Contains(res.Body.String(), sourceErr.Error()) {
		t.Fatalf("response leaked internal error: %s", res.Body.String())
	}
	logLine := logs.String()
	if !strings.Contains(logLine, sourceErr.Error()) {
		t.Fatalf("log does not contain original error: %s", logLine)
	}
	if !strings.Contains(logLine, `"trace_id":"`+traceID.String()+`"`) {
		t.Fatalf("log does not contain trace id: %s", logLine)
	}
	if strings.Count(logLine, sourceErr.Error()) != 1 {
		t.Fatalf("original error logged %d times, want once: %s", strings.Count(logLine, sourceErr.Error()), logLine)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

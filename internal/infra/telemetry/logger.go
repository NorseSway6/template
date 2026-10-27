package telemetry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: want debug, info, warn, or error", value)
	}
}

func NewJSONLogger(level slog.Level, service string, output io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				attr.Value = slog.TimeValue(attr.Value.Time().UTC())
			}
			return attr
		},
	})
	return slog.New(contextHandler{next: handler, service: service})
}

type contextHandler struct {
	next    slog.Handler
	service string
}

func (h contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	spanContext := trace.SpanContextFromContext(ctx)
	traceID, spanID := "", ""
	if spanContext.IsValid() {
		traceID = spanContext.TraceID().String()
		spanID = spanContext.SpanID().String()
	}
	record.AddAttrs(
		slog.String("service", h.service),
		slog.String("trace_id", traceID),
		slog.String("span_id", spanID),
	)
	return h.next.Handle(ctx, record)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{next: h.next.WithAttrs(attrs), service: h.service}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{next: h.next.WithGroup(name), service: h.service}
}

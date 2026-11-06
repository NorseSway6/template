package postgres

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/dbconv"
	"go.opentelemetry.io/otel/trace"
)

const pgxOperationType = "pgx.operation.type"

var collectionPattern = regexp.MustCompile(`(?i)\b(?:FROM|INTO|UPDATE|JOIN)\s+(?:"([^"]+)"|([a-z_][a-z0-9_$]*))(?:\s*\.\s*(?:"([^"]+)"|([a-z_][a-z0-9_$]*)))?`)

type queryTracer struct {
	base     *otelpgx.Tracer
	duration dbconv.ClientOperationDuration
	errors   metric.Int64Counter
}

type queryStart struct {
	at         time.Time
	operation  string
	collection string
}

type queryStartKey struct{}

// newQueryTracer создает новый экземпляр queryTracer с использованием предоставленных провайдеров трассировки и метрик.
func newQueryTracer(tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) (*queryTracer, error) {
	base := otelpgx.NewTracer(
		otelpgx.WithTracerProvider(tracerProvider),
		otelpgx.WithMeterProvider(noop.NewMeterProvider()),
		otelpgx.WithMetricOperationName(otelpgx.SQLOperationName),
	)
	meter := meterProvider.Meter("github.com/NorseSway6/template.git/internal/infra/postgres")
	duration, err := dbconv.NewClientOperationDuration(meter)
	if err != nil {
		return nil, err
	}
	errorsCounter, err := meter.Int64Counter(
		"db.client.operation.errors",
		metric.WithDescription("The count of database client operation errors"),
	)
	if err != nil {
		return nil, err
	}
	return &queryTracer{base: base, duration: duration, errors: errorsCounter}, nil
}

// newDefaultQueryTracer создает новый экземпляр queryTracer с использованием глобальных провайдеров трассировки и метрик.
func newDefaultQueryTracer() (*queryTracer, error) {
	return newQueryTracer(otel.GetTracerProvider(), otel.GetMeterProvider())
}

// TraceQueryStart обрабатывает событие начала выполнения SQL-запроса и добавляет соответствующие атрибуты к текущему контексту.
func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	operation := otelpgx.SQLOperationName(ctx, data.SQL)
	collection := databaseCollection(data.SQL)
	ctx = context.WithValue(ctx, queryStartKey{}, queryStart{
		at:         time.Now(),
		operation:  operation,
		collection: collection,
	})
	ctx = t.base.TraceQueryStart(ctx, conn, data)
	if collection != "" {
		trace.SpanFromContext(ctx).SetAttributes(semconv.DBCollectionName(collection))
	}
	return ctx
}

// TraceQueryEnd обрабатывает событие завершения выполнения SQL-запроса, записывает продолжительность выполнения и ошибки в метрики.
func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	t.base.TraceQueryEnd(ctx, conn, data)

	start, ok := ctx.Value(queryStartKey{}).(queryStart)
	if !ok {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String(pgxOperationType, "query"),
		attribute.String("db.operation.name", start.operation),
	}
	if start.collection != "" {
		attrs = append(attrs, attribute.String("db.collection.name", start.collection))
	}
	if data.Err != nil {
		attrs = append(attrs, semconv.ErrorType(data.Err))
	}
	t.duration.Record(ctx, time.Since(start.at).Seconds(), dbconv.SystemNamePostgreSQL, attrs...)
	if data.Err != nil && !errors.Is(data.Err, sql.ErrNoRows) {
		t.errors.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// databaseCollection извлекает имя таблицы из SQL-запроса.
func databaseCollection(statement string) string {
	match := collectionPattern.FindStringSubmatch(statement)
	if len(match) == 0 {
		return ""
	}
	var collection string
	quoted := false
	switch {
	case match[3] != "":
		collection = match[3]
		quoted = true
	case match[4] != "":
		collection = match[4]
	case match[1] != "":
		collection = match[1]
		quoted = true
	default:
		collection = match[2]
	}
	if quoted {
		return collection
	}
	return strings.ToLower(collection)
}

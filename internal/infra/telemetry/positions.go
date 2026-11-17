package telemetry

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// PositionMetrics представляет структуру для сбора метрик, связанных с позициями поездок.
type PositionMetrics struct {
	staleChecks      metric.Int64Counter
	scanRows         metric.Int64Histogram
	positionRequests metric.Int64Counter
	requestDuration  metric.Float64Histogram
	positionsSaved   metric.Int64Counter
	queueSize        metric.Int64ObservableGauge
	workersActive    metric.Int64ObservableGauge
	registration     metric.Registration
	queueCount       atomic.Int64
	workerCount      atomic.Int64
}

// NewPositionMetrics создает новый экземпляр PositionMetrics с заданными счетчиками и гистограммами метрик.
func NewPositionMetrics() (*PositionMetrics, error) {
	meter := otel.Meter("tripgo")
	m := &PositionMetrics{}
	var err error
	if m.staleChecks, err = meter.Int64Counter("tripgo.stale_checks"); err != nil {
		return nil, err
	}
	if m.scanRows, err = meter.Int64Histogram("tripgo.stale_scan.rows", metric.WithUnit("{row}")); err != nil {
		return nil, err
	}
	if m.positionRequests, err = meter.Int64Counter("tripgo.position_requests"); err != nil {
		return nil, err
	}
	if m.requestDuration, err = meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s")); err != nil {
		return nil, err
	}
	if m.positionsSaved, err = meter.Int64Counter("tripgo.positions.saved"); err != nil {
		return nil, err
	}
	if m.queueSize, err = meter.Int64ObservableGauge("tripgo.position_requests.queue.size"); err != nil {
		return nil, err
	}
	if m.workersActive, err = meter.Int64ObservableGauge("tripgo.position_requests.workers.active"); err != nil {
		return nil, err
	}

	m.registration, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(m.queueSize, m.queueCount.Load())
		observer.ObserveInt64(m.workersActive, m.workerCount.Load())
		return nil
	}, m.queueSize, m.workersActive)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *PositionMetrics) StaleCheck(ctx context.Context, result string) {
	m.staleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (m *PositionMetrics) ScanRows(ctx context.Context, rows int64) {
	m.scanRows.Record(ctx, rows)
}

func (m *PositionMetrics) PositionRequestDropped(ctx context.Context) {
	m.PositionRequest(ctx, "dropped")
}

func (m *PositionMetrics) PositionRequest(ctx context.Context, result string) {
	m.positionRequests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (m *PositionMetrics) PositionRequestDuration(ctx context.Context, duration time.Duration) {
	m.requestDuration.Record(ctx, duration.Seconds())
}

func (m *PositionMetrics) PositionSaved(ctx context.Context) {
	m.positionsSaved.Add(ctx, 1)
}

func (m *PositionMetrics) QueueSize(delta int64) {
	m.queueCount.Add(delta)
}

func (m *PositionMetrics) WorkerActive(delta int64) {
	m.workerCount.Add(delta)
}

func (m *PositionMetrics) Shutdown() error {
	return m.registration.Unregister()
}

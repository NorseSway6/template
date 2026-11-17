package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// TripMetrics представляет структуру для сбора метрик, связанных с поездками.
type TripMetrics struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
}

// NewTripMetrics создает новый экземпляр TripMetrics с заданными счетчиками метрик.
func NewTripMetrics() (*TripMetrics, error) {
	m := otel.Meter("tripgo")
	created, err := m.Int64Counter("tripgo.trips.created")
	if err != nil {
		return nil, err
	}
	completed, err := m.Int64Counter("tripgo.trips.completed")
	if err != nil {
		return nil, err
	}
	return &TripMetrics{created: created, completed: completed}, nil
}

// TripCreated увеличивает счетчик созданных поездок на 1.
func (t *TripMetrics) TripCreated(ctx context.Context) {
	t.created.Add(ctx, 1)
}

// TripCompleted увеличивает счетчик завершенных поездок на 1.
func (t *TripMetrics) TripCompleted(ctx context.Context) {
	t.completed.Add(ctx, 1)
}

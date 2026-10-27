package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type TripMetrics struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
}

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

func (t *TripMetrics) TripCreated()   { t.created.Add(context.Background(), 1) }
func (t *TripMetrics) TripCompleted() { t.completed.Add(context.Background(), 1) }

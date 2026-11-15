package positions

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/NorseSway6/template.git/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=positions

type (
	tripSource interface {
		ListStale(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error)
	}

	ticker interface {
		C() <-chan time.Time
		Stop()
	}

	enqueueQueue interface {
		TryEnqueue(trip model.StaleTrip, checkSpan trace.SpanContext) Result
	}

	staleQueue interface {
		enqueueQueue
		Take(ctx context.Context) (entry, bool)
		Done(e entry)
	}

	gateway interface {
		RequestPosition(ctx context.Context, tripID, driverID string) error
	}

	scannerMetrics interface {
		StaleCheck(ctx context.Context, result string)
		PositionRequestDropped(ctx context.Context)
	}

	poolMetrics interface {
		WorkerActive(delta int64)
		PositionRequest(ctx context.Context, result string)
		PositionRequestDuration(ctx context.Context, duration time.Duration)
	}

	queueMetrics interface {
		QueueSize(delta int64)
	}
)

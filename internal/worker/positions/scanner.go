package positions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Result describes the outcome of an enqueue attempt.
type Result string

const (
	ResultDuplicate Result = "duplicate"
	ResultQueued    Result = "queued"
	ResultDropped   Result = "dropped"
)

type Scanner struct {
	source     tripSource
	queue      enqueueQueue
	ticker     ticker
	staleAfter time.Duration
	metrics    scannerMetrics
	logger     *slog.Logger
	tracer     trace.Tracer
	stopOnce   sync.Once
}

// NewScanner creates a scanner. The ticker is injected so tests can control time.
func NewScanner(
	source tripSource,
	queue enqueueQueue,
	ticker ticker,
	staleAfter time.Duration,
	metrics scannerMetrics,
	logger *slog.Logger,
	tracer trace.Tracer,
) (*Scanner, error) {
	if source == nil {
		return nil, errors.New("position scanner source is nil")
	}
	if queue == nil {
		return nil, errors.New("position scanner queue is nil")
	}
	if ticker == nil {
		return nil, errors.New("position scanner ticker is nil")
	}
	if staleAfter <= 0 {
		return nil, fmt.Errorf("position scanner staleAfter must be positive, got %s", staleAfter)
	}
	if metrics == nil {
		return nil, errors.New("position scanner metrics are nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if tracer == nil {
		tracer = otel.Tracer("tripgo/worker/positions")
	}

	return &Scanner{
		source:     source,
		queue:      queue,
		ticker:     ticker,
		staleAfter: staleAfter,
		metrics:    metrics,
		logger:     logger,
		tracer:     tracer,
	}, nil
}

// Run scans stale trips on each tick until the context is cancelled.
func (s *Scanner) Run(ctx context.Context) {
	defer s.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-s.ticker.C():
			if !ok {
				return
			}
			s.scan(ctx)
		}
	}
}

func (s *Scanner) Stop() {
	s.stopOnce.Do(s.ticker.Stop)
}

func (s *Scanner) scan(ctx context.Context) {
	ctx, span := s.tracer.Start(ctx, "positions.scan")
	defer span.End()

	trips, err := s.source.ListStale(ctx, s.staleAfter)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		s.logger.ErrorContext(ctx, "list stale trips failed", "error", err)
		return
	}

	checkSpan := span.SpanContext()
	for _, staleTrip := range trips {
		if ctx.Err() != nil {
			return
		}

		result := s.queue.TryEnqueue(staleTrip, checkSpan)
		s.metrics.StaleCheck(ctx, string(result))
		if result == ResultDropped {
			s.metrics.PositionRequestDropped(ctx)
			s.logger.WarnContext(ctx, "position queue full, trip dropped", "trip_id", staleTrip.ID)
		}
	}
}

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

type Pool struct {
	queue          staleQueue
	gateway        gateway
	workerCount    int
	requestTimeout time.Duration
	metrics        poolMetrics
	logger         *slog.Logger
	tracer         trace.Tracer
}

func NewPool(
	queue *Queue,
	gateway gateway,
	workerCount int,
	requestTimeout time.Duration,
	metrics poolMetrics,
	logger *slog.Logger,
	tracer trace.Tracer,
) (*Pool, error) {
	if queue == nil {
		return nil, errors.New("position pool queue is nil")
	}
	if gateway == nil {
		return nil, errors.New("position pool gateway is nil")
	}
	if workerCount <= 0 {
		return nil, fmt.Errorf("position pool worker count must be positive, got %d", workerCount)
	}
	if requestTimeout <= 0 {
		return nil, fmt.Errorf("position pool request timeout must be positive, got %s", requestTimeout)
	}
	if metrics == nil {
		return nil, errors.New("position pool metrics are nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if tracer == nil {
		tracer = otel.Tracer("tripgo/worker/positions")
	}

	return &Pool{
		queue:          queue,
		gateway:        gateway,
		workerCount:    workerCount,
		requestTimeout: requestTimeout,
		metrics:        metrics,
		logger:         logger,
		tracer:         tracer,
	}, nil
}

func (p *Pool) Run(ctx context.Context) {
	var workers sync.WaitGroup
	workers.Add(p.workerCount)
	for range p.workerCount {
		go func() {
			defer workers.Done()
			p.runWorker(ctx)
		}()
	}
	workers.Wait()
}

func (p *Pool) runWorker(ctx context.Context) {
	p.metrics.WorkerActive(1)
	defer p.metrics.WorkerActive(-1)

	for {
		e, ok := p.queue.Take(ctx)
		if !ok {
			return
		}
		p.handle(ctx, e)
	}
}

func (p *Pool) handle(ctx context.Context, e entry) {
	defer p.queue.Done(e)

	ctx, span := p.tracer.Start(ctx, "positions.push",
		trace.WithLinks(trace.Link{SpanContext: e.link}))
	defer span.End()

	tctx, cancel := context.WithTimeout(ctx, p.requestTimeout)
	defer cancel()

	startedAt := time.Now()
	err := p.gateway.RequestPosition(tctx, e.trip.ID, e.trip.DriverID)
	p.metrics.PositionRequestDuration(ctx, time.Since(startedAt))
	if err != nil {
		p.metrics.PositionRequest(ctx, "error")
		p.logger.WarnContext(ctx, "push request failed", "trip_id", e.trip.ID, "error", err)
		return
	}
	p.metrics.PositionRequest(ctx, "success")
}

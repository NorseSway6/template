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

	"github.com/NorseSway6/template.git/internal/model"
)

// Result представляет результат попытки добавления устаревшей поездки в очередь.
type Result string

const (
	ResultDuplicate Result = "duplicate"
	ResultQueued    Result = "queued"
	ResultReplaced  Result = "replaced"
	ResultDropped   Result = "dropped"
)

type Scanner struct {
	source     tripSource
	queue      enqueueQueue
	ticker     ticker
	staleAfter time.Duration
	batchSize  int
	metrics    scannerMetrics
	logger     *slog.Logger
	tracer     trace.Tracer
	stopOnce   sync.Once
}

// NewScanner создает новый экземпляр Scanner с указанными параметрами.
func NewScanner(
	source tripSource,
	queue enqueueQueue,
	ticker ticker,
	staleAfter time.Duration,
	batchSize int,
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
	if batchSize <= 0 {
		return nil, fmt.Errorf("position scanner batch size must be positive, got %d", batchSize)
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
		batchSize:  batchSize,
		metrics:    metrics,
		logger:     logger,
		tracer:     tracer,
	}, nil
}

// Run запускает процесс сканирования устаревших поездок и добавления их в очередь.
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

// Stop останавливает процесс сканирования и закрывает тикер.
func (s *Scanner) Stop() {
	s.stopOnce.Do(s.ticker.Stop)
}

// scan выполняет сканирование устаревших поездок и добавляет их в очередь.
func (s *Scanner) scan(ctx context.Context) {
	ctx, span := s.tracer.Start(ctx, "positions.scan")
	defer span.End()

	checkSpan := span.SpanContext()
	var rowsRead int64
	defer func() {
		s.metrics.ScanRows(ctx, rowsRead)
	}()

	var after *model.StaleTrip
	// цикл для получения и обработки устаревших поездок пакетами
	for {
		if ctx.Err() != nil {
			return
		}

		trips, err := s.source.ListStaleBatch(ctx, s.staleAfter, after, s.batchSize)
		if err != nil {
			if ctx.Err() == nil {
				s.logger.ErrorContext(ctx, "list stale trips batch failed", "error", err)
			}
			return
		}
		rowsRead += int64(len(trips))

		for i := range trips {
			if ctx.Err() != nil {
				return
			}

			staleTrip := trips[i]
			result := s.queue.TryEnqueue(staleTrip, checkSpan)
			switch result {
			case ResultDuplicate:
				s.metrics.StaleCheck(ctx, string(ResultDuplicate))
			case ResultQueued:
				s.metrics.StaleCheck(ctx, string(ResultQueued))
			case ResultDropped:
				s.metrics.StaleCheck(ctx, string(ResultDropped))
				s.metrics.PositionRequestDropped(ctx)
				s.logger.WarnContext(ctx, "position queue full, trip dropped", "trip_id", staleTrip.ID)
			case ResultReplaced:
				s.metrics.StaleCheck(ctx, string(ResultQueued))
				s.metrics.StaleCheck(ctx, string(ResultDropped))
				s.metrics.PositionRequestDropped(ctx)
				s.logger.WarnContext(ctx, "position queue full, fresher trip evicted for older trip",
					"trip_id", staleTrip.ID)
			}
			if s.queue.Full() && (result == ResultDropped || result == ResultReplaced) {
				return
			}
			after = &staleTrip
		}

		if len(trips) < s.batchSize || s.queue.Full() {
			return
		}
	}
}

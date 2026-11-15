package positions

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/otel/trace"

	"github.com/NorseSway6/template.git/internal/model"
)

type entry struct {
	trip model.StaleTrip
	seq  int64
	link trace.SpanContext
}

type queueItem struct {
	seq        int64
	processing bool
}

type Queue struct {
	mu      sync.Mutex
	pending map[string]queueItem
	ch      chan entry
	seq     int64
	closed  bool
	metrics queueMetrics
}

func NewQueue(capacity int, metrics queueMetrics) (*Queue, error) {
	if capacity <= 0 {
		return nil, errors.New("position queue capacity must be positive")
	}
	if metrics == nil {
		return nil, errors.New("position queue metrics are nil")
	}
	return &Queue{
		pending: make(map[string]queueItem),
		ch:      make(chan entry, capacity),
		metrics: metrics,
	}, nil
}

func (q *Queue) TryEnqueue(t model.StaleTrip, link trace.SpanContext) Result {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ResultDropped
	}
	if _, ok := q.pending[t.ID]; ok {
		return ResultDuplicate
	}
	q.seq++
	select {
	case q.ch <- entry{trip: t, seq: q.seq, link: link}:
		q.pending[t.ID] = queueItem{seq: q.seq}
		q.metrics.QueueSize(1)
		return ResultQueued
	default:
		return ResultDropped
	}
}

func (q *Queue) Cancel(tripID string) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	item, ok := q.pending[tripID]
	if ok && !item.processing {
		delete(q.pending, tripID)
		queued := make([]entry, 0, len(q.ch))
		for {
			select {
			case e := <-q.ch:
				if e.trip.ID == tripID && e.seq == item.seq {
					q.metrics.QueueSize(-1)
					continue
				}
				queued = append(queued, e)
			default:
				for _, e := range queued {
					q.ch <- e
				}
				q.mu.Unlock()
				return
			}
		}
	}
	q.mu.Unlock()
}

func (q *Queue) Take(ctx context.Context) (entry, bool) {
	for {
		select {
		case <-ctx.Done():
			return entry{}, false
		case e, ok := <-q.ch:
			if !ok {
				return entry{}, false
			}
			q.mu.Lock()
			q.metrics.QueueSize(-1)
			item, ok := q.pending[e.trip.ID]
			if ok && item.seq == e.seq && !item.processing {
				item.processing = true
				q.pending[e.trip.ID] = item
				q.mu.Unlock()
				return e, true
			}
			q.mu.Unlock()
		}
	}
}

func (q *Queue) Done(e entry) {
	q.mu.Lock()
	if item, ok := q.pending[e.trip.ID]; ok && item.seq == e.seq {
		delete(q.pending, e.trip.ID)
	}
	q.mu.Unlock()
}

func (q *Queue) Close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
	q.mu.Unlock()
}

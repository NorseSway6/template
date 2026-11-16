package positions

import (
	"container/heap"
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

type entryHeap []entry

// Len возвращает количество элементов в куче.
func (h entryHeap) Len() int { return len(h) }

// Less определяет порядок элементов в куче.
func (h entryHeap) Less(i, j int) bool {
	if h[i].trip.StaleSince.Equal(h[j].trip.StaleSince) {
		return h[i].seq < h[j].seq
	}
	return h[i].trip.StaleSince.Before(h[j].trip.StaleSince)
}

// Swap меняет местами элементы в куче.
func (h entryHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push добавляет элемент в кучу.
func (h *entryHeap) Push(value any) { *h = append(*h, value.(entry)) }

// Pop удаляет и возвращает наименьший элемент из кучи.
func (h *entryHeap) Pop() any {
	old := *h
	last := len(old) - 1
	value := old[last]
	*h = old[:last]
	return value
}

type queueItem struct {
	seq        int64
	processing bool
}

// Queue представляет очередь позиций, которая хранит позиции в порядке их устаревания.
type Queue struct {
	mu       sync.Mutex
	pending  map[string]queueItem
	items    entryHeap
	capacity int
	wake     chan struct{}
	seq      int64
	closed   bool
	metrics  queueMetrics
}

// NewQueue создает новую очередь позиций.
func NewQueue(capacity int, metrics queueMetrics) (*Queue, error) {
	if capacity <= 0 {
		return nil, errors.New("position queue capacity must be positive")
	}
	if metrics == nil {
		return nil, errors.New("position queue metrics are nil")
	}
	return &Queue{
		pending:  make(map[string]queueItem),
		capacity: capacity,
		wake:     make(chan struct{}, 1),
		metrics:  metrics,
	}, nil
}

// TryEnqueue пытается добавить устаревшую поездку в очередь.
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
	newEntry := entry{trip: t, seq: q.seq, link: link}
	if len(q.items) < q.capacity {
		heap.Push(&q.items, newEntry)
		q.pending[t.ID] = queueItem{seq: q.seq}
		q.metrics.QueueSize(1)
		q.signal()
		return ResultQueued
	}

	newestIndex := q.newestIndex()
	if !q.entryHasHigherPriority(newEntry, q.items[newestIndex]) {
		return ResultDropped
	}

	evicted := heap.Remove(&q.items, newestIndex).(entry)
	delete(q.pending, evicted.trip.ID)
	heap.Push(&q.items, newEntry)
	q.pending[t.ID] = queueItem{seq: q.seq}
	q.signal()
	return ResultReplaced
}

// newestIndex возвращает индекс элемента с наименьшим приоритетом в очереди.
func (q *Queue) newestIndex() int {
	newest := 0
	for i := 1; i < len(q.items); i++ {
		if q.entryHasHigherPriority(q.items[newest], q.items[i]) {
			newest = i
		}
	}
	return newest
}

// entryHasHigherPriority возвращает true, если элемент a имеет более высокий приоритет, чем элемент b.
func (q *Queue) entryHasHigherPriority(a, b entry) bool {
	if a.trip.StaleSince.Equal(b.trip.StaleSince) {
		return a.seq < b.seq
	}
	return a.trip.StaleSince.Before(b.trip.StaleSince)
}

// Full возвращает true, если очередь закрыта или достигла своей емкости.
func (q *Queue) Full() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed || len(q.items) >= q.capacity
}

// Cancel удаляет устаревшую поездку из очереди, если она еще не обрабатывается.
func (q *Queue) Cancel(tripID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	item, ok := q.pending[tripID]
	if !ok || item.processing {
		return
	}
	for i, queued := range q.items {
		if queued.trip.ID == tripID && queued.seq == item.seq {
			heap.Remove(&q.items, i)
			delete(q.pending, tripID)
			q.metrics.QueueSize(-1)
			return
		}
	}
}

// Take извлекает устаревшую поездку из очереди для обработки.
func (q *Queue) Take(ctx context.Context) (entry, bool) {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			e := heap.Pop(&q.items).(entry)
			item := q.pending[e.trip.ID]
			item.processing = true
			q.pending[e.trip.ID] = item
			q.metrics.QueueSize(-1)
			if len(q.items) > 0 && !q.closed {
				q.signal()
			}
			q.mu.Unlock()
			return e, true
		}
		if q.closed {
			q.mu.Unlock()
			return entry{}, false
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return entry{}, false
		case <-q.wake:
		}
	}
}

// Done помечает устаревшую поездку как обработанную и удаляет ее из очереди.
func (q *Queue) Done(e entry) {
	q.mu.Lock()
	if item, ok := q.pending[e.trip.ID]; ok && item.seq == e.seq {
		delete(q.pending, e.trip.ID)
	}
	q.mu.Unlock()
}

// Close закрывает очередь и сигнализирует всем ожидающим горутинам.
func (q *Queue) Close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.wake)
	}
	q.mu.Unlock()
}

// signal отправляет сигнал в канал wake, чтобы разбудить ожидающие горутины.
func (q *Queue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

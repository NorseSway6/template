package positions

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/model"
)

func TestQueueCapacityDeduplicationAndCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockqueueMetrics(ctrl)
	metrics.EXPECT().QueueSize(int64(1)).Times(2)
	metrics.EXPECT().QueueSize(int64(-1)).Times(2)
	q, err := NewQueue(1, metrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}

	first := model.StaleTrip{ID: "trip-1"}
	second := model.StaleTrip{ID: "trip-2"}
	if got := q.TryEnqueue(first, trace.SpanContext{}); got != ResultQueued {
		t.Fatalf("first TryEnqueue() = %q, want %q", got, ResultQueued)
	}
	if got := q.TryEnqueue(first, trace.SpanContext{}); got != ResultDuplicate {
		t.Fatalf("duplicate TryEnqueue() = %q, want %q", got, ResultDuplicate)
	}
	if got := q.TryEnqueue(second, trace.SpanContext{}); got != ResultDropped {
		t.Fatalf("full queue TryEnqueue() = %q, want %q", got, ResultDropped)
	}

	q.Cancel(first.ID)
	if got := q.TryEnqueue(second, trace.SpanContext{}); got != ResultQueued {
		t.Fatalf("TryEnqueue() after cancellation = %q, want %q", got, ResultQueued)
	}

	item, ok := q.Take(context.Background())
	if !ok {
		t.Fatal("Take() returned false for queued trip")
	}
	if item.trip.ID != second.ID {
		t.Fatalf("Take() trip ID = %q, want %q", item.trip.ID, second.ID)
	}
	q.Done(item)
	q.Close()
	if _, ok := q.Take(context.Background()); ok {
		t.Fatal("Take() returned a trip after the closed queue was drained")
	}
}

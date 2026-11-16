package positions

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/model"
)

const (
	middleTripID   = "middle"
	freshestTripID = "freshest"
	oldestTripID   = "oldest"
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

func TestQueueTakesTripsByOldestPositionFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockqueueMetrics(ctrl)
	metrics.EXPECT().QueueSize(int64(1)).Times(3)
	metrics.EXPECT().QueueSize(int64(-1)).Times(3)
	q, err := NewQueue(3, metrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}

	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	trips := []model.StaleTrip{
		{ID: middleTripID, StaleSince: base.Add(-time.Hour)},
		{ID: freshestTripID, StaleSince: base},
		{ID: oldestTripID, StaleSince: base.Add(-2 * time.Hour)},
	}
	for _, trip := range trips {
		if got := q.TryEnqueue(trip, trace.SpanContext{}); got != ResultQueued {
			t.Fatalf("TryEnqueue(%q) = %q, want %q", trip.ID, got, ResultQueued)
		}
	}

	for _, want := range []string{oldestTripID, middleTripID, freshestTripID} {
		item, ok := q.Take(context.Background())
		if !ok {
			t.Fatalf("Take() returned false before trip %q", want)
		}
		if item.trip.ID != want {
			t.Fatalf("Take() trip ID = %q, want %q", item.trip.ID, want)
		}
		q.Done(item)
	}
	q.Close()
}

func TestQueueReplacesNewestQueuedTripWhenFull(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockqueueMetrics(ctrl)
	metrics.EXPECT().QueueSize(int64(1)).Times(2)
	metrics.EXPECT().QueueSize(int64(-1)).Times(2)
	q, err := NewQueue(2, metrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}

	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, trip := range []model.StaleTrip{
		{ID: middleTripID, StaleSince: base.Add(-time.Hour)},
		{ID: freshestTripID, StaleSince: base},
	} {
		if got := q.TryEnqueue(trip, trace.SpanContext{}); got != ResultQueued {
			t.Fatalf("TryEnqueue(%q) = %q, want %q", trip.ID, got, ResultQueued)
		}
	}
	oldest := model.StaleTrip{ID: oldestTripID, StaleSince: base.Add(-2 * time.Hour)}
	if got := q.TryEnqueue(oldest, trace.SpanContext{}); got != ResultReplaced {
		t.Fatalf("TryEnqueue(oldest) = %q, want %q", got, ResultReplaced)
	}

	for _, want := range []string{oldestTripID, middleTripID} {
		item, ok := q.Take(context.Background())
		if !ok {
			t.Fatalf("Take() returned false before trip %q", want)
		}
		if item.trip.ID != want {
			t.Fatalf("Take() trip ID = %q, want %q", item.trip.ID, want)
		}
		q.Done(item)
	}
	q.Close()
}

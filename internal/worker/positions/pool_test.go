package positions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/model"
)

const tripAfterErrorID = "trip-after-error"

func TestPoolRunsConfiguredWorkersAndContinuesAfterGatewayError(t *testing.T) {
	ctrl := gomock.NewController(t)
	queueMetrics := NewMockqueueMetrics(ctrl)
	metrics := NewMockpoolMetrics(ctrl)
	gateway := NewMockgateway(ctrl)
	started := make(chan string, 4)
	release := make(chan struct{}, 4)
	results := make(chan string, 4)
	var active atomic.Int64
	var maxActive atomic.Int64

	queueMetrics.EXPECT().QueueSize(int64(1)).Times(4)
	queueMetrics.EXPECT().QueueSize(int64(-1)).Times(4)
	metrics.EXPECT().WorkerActive(int64(1)).Times(3)
	metrics.EXPECT().WorkerActive(int64(-1)).Times(3)
	metrics.EXPECT().PositionRequestDuration(gomock.Any(), gomock.Any()).Times(4)
	metrics.EXPECT().PositionRequest(gomock.Any(), gomock.Any()).Do(func(_ context.Context, result string) {
		results <- result
	}).Times(4)

	for _, tripID := range []string{"1", "2", "3", tripAfterErrorID} {
		tripID := tripID
		gateway.EXPECT().RequestPosition(gomock.Any(), tripID, "").DoAndReturn(
			func(context.Context, string, string) error {
				current := active.Add(1)
				for {
					previous := maxActive.Load()
					if current <= previous || maxActive.CompareAndSwap(previous, current) {
						break
					}
				}
				defer active.Add(-1)
				started <- tripID
				<-release
				if tripID == "1" {
					return errors.New("push service unavailable")
				}
				return nil
			},
		)
	}

	queue, err := NewQueue(4, queueMetrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	pool, err := NewPool(
		queue,
		gateway,
		3,
		time.Second,
		metrics,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
	)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		pool.Run(ctx)
	}()

	for _, tripID := range []string{"1", "2", "3"} {
		if got := queue.TryEnqueue(model.StaleTrip{ID: tripID}, trace.SpanContext{}); got != ResultQueued {
			t.Fatalf("TryEnqueue() = %q, want %q", got, ResultQueued)
		}
	}
	seen := map[string]bool{}
	for range 3 {
		select {
		case tripID := <-started:
			seen[tripID] = true
		case <-time.After(time.Second):
			t.Fatal("configured worker pool did not start three concurrent requests")
		}
	}
	if got := maxActive.Load(); got != 3 {
		t.Fatalf("maximum concurrent requests = %d, want 3", got)
	}
	if len(seen) != 3 {
		t.Fatalf("started trip IDs = %v, want three unique trips", seen)
	}

	for range 3 {
		release <- struct{}{}
	}
	for range 3 {
		select {
		case <-results:
		case <-time.After(time.Second):
			t.Fatal("worker did not record push result")
		}
	}

	if got := queue.TryEnqueue(model.StaleTrip{ID: tripAfterErrorID}, trace.SpanContext{}); got != ResultQueued {
		t.Fatalf("TryEnqueue() after failed push = %q, want %q", got, ResultQueued)
	}
	select {
	case tripID := <-started:
		if tripID != tripAfterErrorID {
			t.Fatalf("next request trip ID = %q, want %s", tripID, tripAfterErrorID)
		}
	case <-time.After(time.Second):
		t.Fatal("pool stopped processing after gateway failure")
	}
	release <- struct{}{}
	select {
	case result := <-results:
		if result != "success" {
			t.Fatalf("request result after failure = %q, want success", result)
		}
	case <-time.After(time.Second):
		t.Fatal("pool did not process the next request")
	}

	queue.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker pool did not stop after queue drain")
	}
}

func TestPoolStopsWorkersAfterContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	queueMetrics := NewMockqueueMetrics(ctrl)
	metrics := NewMockpoolMetrics(ctrl)
	started := make(chan struct{}, 2)

	metrics.EXPECT().WorkerActive(int64(1)).Do(func(int64) { started <- struct{}{} }).Times(2)
	metrics.EXPECT().WorkerActive(int64(-1)).Times(2)

	queue, err := NewQueue(1, queueMetrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	pool, err := NewPool(
		queue, NewMockgateway(ctrl), 2, time.Second, metrics, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		pool.Run(ctx)
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker pool did not start both workers")
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker pool did not stop after context cancellation")
	}
	queue.Close()
}

func TestQueuedPositionCancellationPreventsPush(t *testing.T) {
	ctrl := gomock.NewController(t)
	queueMetrics := NewMockqueueMetrics(ctrl)
	queueMetrics.EXPECT().QueueSize(int64(1))
	queueMetrics.EXPECT().QueueSize(int64(-1))

	queue, err := NewQueue(1, queueMetrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	trip := model.StaleTrip{ID: "trip-with-new-position"}
	if got := queue.TryEnqueue(trip, trace.SpanContext{}); got != ResultQueued {
		t.Fatalf("TryEnqueue() = %q, want %q", got, ResultQueued)
	}
	queue.Cancel(trip.ID)
	queue.Close()

	metrics := NewMockpoolMetrics(ctrl)
	metrics.EXPECT().WorkerActive(int64(1))
	metrics.EXPECT().WorkerActive(int64(-1))
	gateway := NewMockgateway(ctrl)
	gateway.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	pool, err := NewPool(queue, gateway, 1, time.Second, metrics, nil, nil)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	pool.Run(context.Background())
}

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
	durationResults := make(chan string, 4)
	workerChanges := make(chan int64, 8)
	var active atomic.Int64
	var maxActive atomic.Int64
	var activeWorkers atomic.Int64

	queueMetrics.EXPECT().QueueSize(int64(1)).Times(4)
	queueMetrics.EXPECT().QueueSize(int64(-1)).Times(4)
	metrics.EXPECT().WorkerActive(gomock.Any()).Do(func(delta int64) {
		activeWorkers.Add(delta)
		workerChanges <- delta
	}).Times(8)
	metrics.EXPECT().PositionRequestDuration(gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, _ time.Duration, result string) {
			durationResults <- result
		}).Times(4)
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
	seen := waitForStartedTrips(t, started, 3)
	if got := maxActive.Load(); got != 3 {
		t.Fatalf("maximum concurrent requests = %d, want 3", got)
	}
	if len(seen) != 3 {
		t.Fatalf("started trip IDs = %v, want three unique trips", seen)
	}
	waitForActiveWorkers(t, workerChanges, &activeWorkers, 3)

	releaseRequests(release, 3)
	waitForValues(t, results, 3, "worker did not record push result")
	waitForDurationOutcomes(t, durationResults, 3, map[string]int{"error": 1, "success": 2})

	if got := queue.TryEnqueue(model.StaleTrip{ID: tripAfterErrorID}, trace.SpanContext{}); got != ResultQueued {
		t.Fatalf("TryEnqueue() after failed push = %q, want %q", got, ResultQueued)
	}
	waitForTripID(t, started, tripAfterErrorID)
	releaseRequests(release, 1)
	waitForResult(t, results, "success", "pool did not process the next request")
	waitForResult(t, durationResults, "success", "pool did not record duration for the next request")
	waitForInactiveWorkers(t, workerChanges, &activeWorkers)

	queue.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker pool did not stop after queue drain")
	}
}

func waitForStartedTrips(t *testing.T, started <-chan string, count int) map[string]bool {
	t.Helper()
	seen := make(map[string]bool, count)
	for range count {
		select {
		case tripID := <-started:
			seen[tripID] = true
		case <-time.After(time.Second):
			t.Fatal("configured worker pool did not start concurrent requests")
		}
	}
	return seen
}

func waitForActiveWorkers(t *testing.T, changes <-chan int64, active *atomic.Int64, count int) {
	t.Helper()
	for range count {
		select {
		case delta := <-changes:
			if delta != 1 {
				t.Fatalf("WorkerActive() delta = %d, want 1 while starting work", delta)
			}
		case <-time.After(time.Second):
			t.Fatal("worker pool did not record active work")
		}
	}
	if got := active.Load(); got != int64(count) {
		t.Fatalf("active workers while requests are blocked = %d, want %d", got, count)
	}
}

func releaseRequests(release chan<- struct{}, count int) {
	for range count {
		release <- struct{}{}
	}
}

func waitForValues(t *testing.T, values <-chan string, count int, timeoutMessage string) {
	t.Helper()
	for range count {
		select {
		case <-values:
		case <-time.After(time.Second):
			t.Fatal(timeoutMessage)
		}
	}
}

func waitForDurationOutcomes(t *testing.T, outcomes <-chan string, count int, expected map[string]int) {
	t.Helper()
	got := make(map[string]int, len(expected))
	for range count {
		select {
		case result := <-outcomes:
			got[result]++
		case <-time.After(time.Second):
			t.Fatal("worker did not record push duration result")
		}
	}
	for result, want := range expected {
		if got[result] != want {
			t.Fatalf("duration results = %v, want %v", got, expected)
		}
	}
}

func waitForTripID(t *testing.T, started <-chan string, expected string) {
	t.Helper()
	select {
	case tripID := <-started:
		if tripID != expected {
			t.Fatalf("next request trip ID = %q, want %s", tripID, expected)
		}
	case <-time.After(time.Second):
		t.Fatal("pool stopped processing after gateway failure")
	}
}

func waitForResult(t *testing.T, results <-chan string, expected, timeoutMessage string) {
	t.Helper()
	select {
	case result := <-results:
		if result != expected {
			t.Fatalf("result = %q, want %q", result, expected)
		}
	case <-time.After(time.Second):
		t.Fatal(timeoutMessage)
	}
}

func waitForInactiveWorkers(t *testing.T, changes <-chan int64, active *atomic.Int64) {
	t.Helper()
	for active.Load() != 0 {
		select {
		case <-changes:
		case <-time.After(time.Second):
			t.Fatalf("workers remained active after requests finished: %d", active.Load())
		}
	}
}

func TestPoolStopsWorkersAfterContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	queueMetrics := NewMockqueueMetrics(ctrl)
	metrics := NewMockpoolMetrics(ctrl)

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
	gateway := NewMockgateway(ctrl)
	gateway.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	pool, err := NewPool(queue, gateway, 1, time.Second, metrics, nil, nil)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	pool.Run(context.Background())
}

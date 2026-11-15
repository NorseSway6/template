package positions

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/model"
)

func TestScannerRunsOnTickAndRecordsDroppedTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	staleAfter := 2 * time.Minute
	staleTrips := []model.StaleTrip{
		{ID: "trip-queued"},
		{ID: "trip-dropped"},
	}
	source := NewMocktripSource(ctrl)
	ticker := NewMockticker(ctrl)
	queue := NewMockenqueueQueue(ctrl)
	metrics := NewMockscannerMetrics(ctrl)
	ticks := make(chan time.Time)
	results := make(chan string, 2)
	dropped := make(chan struct{}, 1)

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	source.EXPECT().ListStale(gomock.Any(), staleAfter).Return(staleTrips, nil)
	queue.EXPECT().TryEnqueue(staleTrips[0], gomock.Any()).Return(ResultQueued)
	queue.EXPECT().TryEnqueue(staleTrips[1], gomock.Any()).Return(ResultDropped)
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultQueued)).Do(func(context.Context, string) {
		results <- string(ResultQueued)
	})
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultDropped)).Do(func(context.Context, string) {
		results <- string(ResultDropped)
	})
	metrics.EXPECT().PositionRequestDropped(gomock.Any()).Do(func(context.Context) {
		dropped <- struct{}{}
	})

	scanner, err := NewScanner(
		source,
		queue,
		ticker,
		staleAfter,
		metrics,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
	)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner.Run(ctx)
	}()

	ticks <- time.Now()
	for _, want := range []string{string(ResultQueued), string(ResultDropped)} {
		select {
		case got := <-results:
			if got != want {
				t.Fatalf("StaleCheck() result = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("scanner did not record result %q", want)
		}
	}
	select {
	case <-dropped:
	case <-time.After(time.Second):
		t.Fatal("scanner did not record the dropped position request")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after context cancellation")
	}
}

func TestScannerDeduplicatesTripAcrossTicks(t *testing.T) {
	ctrl := gomock.NewController(t)
	staleTrip := model.StaleTrip{ID: "trip-duplicate"}
	source := NewMocktripSource(ctrl)
	ticker := NewMockticker(ctrl)
	metrics := NewMockscannerMetrics(ctrl)
	queueMetrics := NewMockqueueMetrics(ctrl)
	ticks := make(chan time.Time)
	results := make(chan string, 2)

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	source.EXPECT().ListStale(gomock.Any(), time.Minute).Return([]model.StaleTrip{staleTrip}, nil).Times(2)
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultQueued)).Do(func(context.Context, string) {
		results <- string(ResultQueued)
	})
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultDuplicate)).Do(func(context.Context, string) {
		results <- string(ResultDuplicate)
	})
	queueMetrics.EXPECT().QueueSize(int64(1))
	queueMetrics.EXPECT().QueueSize(int64(-1))

	queue, err := NewQueue(1, queueMetrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	scanner, err := NewScanner(
		source,
		queue,
		ticker,
		time.Minute,
		metrics,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
	)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner.Run(ctx)
	}()

	for _, want := range []string{string(ResultQueued), string(ResultDuplicate)} {
		ticks <- time.Now()
		select {
		case got := <-results:
			if got != want {
				t.Fatalf("StaleCheck() result = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("scanner did not record result %q", want)
		}
	}

	item, ok := queue.Take(context.Background())
	if !ok {
		t.Fatal("queue did not contain the first scheduled trip")
	}
	if item.trip.ID != staleTrip.ID {
		t.Fatalf("queued trip ID = %q, want %q", item.trip.ID, staleTrip.ID)
	}
	queue.Done(item)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after context cancellation")
	}
	queue.Close()
}

func TestScannerDropsWhenQueueIsFullWithoutBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMocktripSource(ctrl)
	ticker := NewMockticker(ctrl)
	metrics := NewMockscannerMetrics(ctrl)
	queueMetrics := NewMockqueueMetrics(ctrl)
	ticks := make(chan time.Time)
	results := make(chan string, 2)
	dropped := make(chan struct{}, 1)
	trips := []model.StaleTrip{
		{ID: "trip-queued"},
		{ID: "trip-dropped"},
	}

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	source.EXPECT().ListStale(gomock.Any(), time.Minute).Return(trips, nil)
	queueMetrics.EXPECT().QueueSize(int64(1))
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultQueued)).Do(func(context.Context, string) {
		results <- string(ResultQueued)
	})
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultDropped)).Do(func(context.Context, string) {
		results <- string(ResultDropped)
	})
	metrics.EXPECT().PositionRequestDropped(gomock.Any()).Do(func(context.Context) {
		dropped <- struct{}{}
	})

	queue, err := NewQueue(1, queueMetrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	scanner, err := NewScanner(
		source,
		queue,
		ticker,
		time.Minute,
		metrics,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		nil,
	)
	if err != nil {
		t.Fatalf("NewScanner() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner.Run(ctx)
	}()

	ticks <- time.Now()
	for _, want := range []string{string(ResultQueued), string(ResultDropped)} {
		select {
		case got := <-results:
			if got != want {
				t.Fatalf("StaleCheck() result = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("scanner blocked before recording result %q", want)
		}
	}
	select {
	case <-dropped:
	case <-time.After(time.Second):
		t.Fatal("scanner did not record the dropped request")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after context cancellation")
	}
	queue.Close()
}

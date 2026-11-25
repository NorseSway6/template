package positions

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/model"
)

const (
	scannerTripA = "trip-a"
	scannerTripB = "trip-b"
)

const scannerTestBatchSize = 100

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
	scanDone := make(chan int64, 1)
	dropped := make(chan struct{}, 1)

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	source.EXPECT().ListStaleBatch(gomock.Any(), staleAfter, nil, scannerTestBatchSize).Return(staleTrips, nil)
	queue.EXPECT().TryEnqueue(staleTrips[0], gomock.Any()).Return(ResultQueued)
	queue.EXPECT().Full().Return(false)
	queue.EXPECT().TryEnqueue(staleTrips[1], gomock.Any()).Return(ResultDropped)
	queue.EXPECT().Full().Return(false)
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultQueued)).Do(func(context.Context, string) {
		results <- string(ResultQueued)
	})
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultDropped)).Do(func(context.Context, string) {
		results <- string(ResultDropped)
	})
	metrics.EXPECT().PositionRequestDropped(gomock.Any()).Do(func(context.Context) {
		dropped <- struct{}{}
	})
	metrics.EXPECT().ScanRows(gomock.Any(), int64(2)).Do(func(_ context.Context, rows int64) {
		scanDone <- rows
	})

	scanner, err := NewScanner(
		source,
		queue,
		ticker,
		staleAfter,
		scannerTestBatchSize,
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
	select {
	case rows := <-scanDone:
		if rows != int64(len(staleTrips)) {
			t.Fatalf("ScanRows() = %d, want %d", rows, len(staleTrips))
		}
	case <-time.After(time.Second):
		t.Fatal("scanner did not record rows read")
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
	scans := make(chan struct{}, 2)

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	source.EXPECT().ListStaleBatch(gomock.Any(), time.Minute, nil, scannerTestBatchSize).
		Return([]model.StaleTrip{staleTrip}, nil).Times(2)
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultQueued)).Do(func(context.Context, string) {
		results <- string(ResultQueued)
	})
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultDuplicate)).Do(func(context.Context, string) {
		results <- string(ResultDuplicate)
	})
	metrics.EXPECT().ScanRows(gomock.Any(), int64(1)).Do(func(context.Context, int64) {
		scans <- struct{}{}
	}).Times(2)
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
		scannerTestBatchSize,
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
		select {
		case <-scans:
		case <-time.After(time.Second):
			t.Fatal("scanner did not finish the tick")
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

func TestScannerStopsWhenQueueFills(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMocktripSource(ctrl)
	ticker := NewMockticker(ctrl)
	metrics := NewMockscannerMetrics(ctrl)
	queueMetrics := NewMockqueueMetrics(ctrl)
	ticks := make(chan time.Time)
	scanDone := make(chan int64, 1)
	trips := []model.StaleTrip{
		{ID: "trip-queued"},
		{ID: "trip-not-processed"},
	}

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	source.EXPECT().ListStaleBatch(gomock.Any(), time.Minute, nil, scannerTestBatchSize).
		Return(trips, nil)
	queueMetrics.EXPECT().QueueSize(int64(1))
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultQueued))
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultDropped))
	metrics.EXPECT().PositionRequestDropped(gomock.Any())
	metrics.EXPECT().ScanRows(gomock.Any(), int64(2)).Do(func(_ context.Context, rows int64) {
		scanDone <- rows
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
		scannerTestBatchSize,
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
	select {
	case <-scanDone:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after filling the queue")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after context cancellation")
	}
	queue.Close()
}

func TestScannerRecordsBothDuplicatesWhenQueueIsFull(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMocktripSource(ctrl)
	ticker := NewMockticker(ctrl)
	metrics := NewMockscannerMetrics(ctrl)
	queueMetrics := NewMockqueueMetrics(ctrl)
	ticks := make(chan time.Time)
	scanDone := make(chan struct{}, 1)
	trips := []model.StaleTrip{{ID: scannerTripA}, {ID: scannerTripB}}

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	source.EXPECT().ListStaleBatch(gomock.Any(), time.Minute, nil, scannerTestBatchSize).
		Return(trips, nil)
	queueMetrics.EXPECT().QueueSize(int64(1)).Times(2)
	queueMetrics.EXPECT().QueueSize(int64(-1)).Times(2)
	for range trips {
		metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultDuplicate))
	}
	metrics.EXPECT().ScanRows(gomock.Any(), int64(len(trips))).Do(func(context.Context, int64) {
		scanDone <- struct{}{}
	})

	queue, err := NewQueue(len(trips), queueMetrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	for _, staleTrip := range trips {
		if got := queue.TryEnqueue(staleTrip, trace.SpanContext{}); got != ResultQueued {
			t.Fatalf("TryEnqueue(%q) = %q, want %q", staleTrip.ID, got, ResultQueued)
		}
	}
	scanner, err := NewScanner(
		source,
		queue,
		ticker,
		time.Minute,
		scannerTestBatchSize,
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
	select {
	case <-scanDone:
	case <-time.After(time.Second):
		t.Fatal("scanner did not finish duplicate batch")
	}

	for _, want := range trips {
		item, ok := queue.Take(context.Background())
		if !ok {
			t.Fatalf("queue did not retain trip %q", want.ID)
		}
		if item.trip.ID != want.ID {
			t.Fatalf("Take() trip ID = %q, want %q", item.trip.ID, want.ID)
		}
		queue.Done(item)
	}
	queue.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after context cancellation")
	}
}

func TestScannerCountsReplacedTripAsQueuedAndEvictionAsDropped(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMocktripSource(ctrl)
	ticker := NewMockticker(ctrl)
	metrics := NewMockscannerMetrics(ctrl)
	queueMetrics := NewMockqueueMetrics(ctrl)
	ticks := make(chan time.Time)
	scanDone := make(chan struct{}, 1)
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	staleTrips := []model.StaleTrip{
		{ID: "trip-b", StaleSince: base.Add(-time.Hour)},
		{ID: "trip-c", StaleSince: base},
	}
	olderTrip := model.StaleTrip{ID: "trip-a", StaleSince: base.Add(-2 * time.Hour)}

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	source.EXPECT().ListStaleBatch(gomock.Any(), time.Minute, nil, scannerTestBatchSize).
		Return([]model.StaleTrip{olderTrip}, nil)
	queueMetrics.EXPECT().QueueSize(int64(1)).Times(2)
	queueMetrics.EXPECT().QueueSize(int64(-1)).Times(2)
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultQueued))
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultDropped))
	metrics.EXPECT().PositionRequestDropped(gomock.Any())
	metrics.EXPECT().ScanRows(gomock.Any(), int64(1)).Do(func(context.Context, int64) {
		scanDone <- struct{}{}
	})

	queue, err := NewQueue(2, queueMetrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	for _, staleTrip := range staleTrips {
		if got := queue.TryEnqueue(staleTrip, trace.SpanContext{}); got != ResultQueued {
			t.Fatalf("TryEnqueue(%q) = %q, want %q", staleTrip.ID, got, ResultQueued)
		}
	}
	scanner, err := NewScanner(
		source,
		queue,
		ticker,
		time.Minute,
		scannerTestBatchSize,
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
	select {
	case <-scanDone:
	case <-time.After(time.Second):
		t.Fatal("scanner did not finish replacement batch")
	}

	for _, wantID := range []string{scannerTripA, scannerTripB} {
		item, ok := queue.Take(context.Background())
		if !ok {
			t.Fatalf("queue did not contain trip %q", wantID)
		}
		if item.trip.ID != wantID {
			t.Fatalf("Take() trip ID = %q, want %q", item.trip.ID, wantID)
		}
		queue.Done(item)
	}
	queue.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after context cancellation")
	}
}

func TestScannerReadsAllStaleTripsInKeysetBatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	source := NewMocktripSource(ctrl)
	ticker := NewMockticker(ctrl)
	metrics := NewMockscannerMetrics(ctrl)
	queueMetrics := NewMockqueueMetrics(ctrl)
	ticks := make(chan time.Time)
	scanDone := make(chan int64, 1)
	const tripCount = 250
	const batchSize = 100
	trips := make([]model.StaleTrip, tripCount)
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := range trips {
		trips[i] = model.StaleTrip{
			ID:         fmt.Sprintf("trip-%03d", i),
			StaleSince: base.Add(time.Duration(i) * time.Second),
		}
	}

	ticker.EXPECT().C().AnyTimes().Return(ticks)
	ticker.EXPECT().Stop()
	queueMetrics.EXPECT().QueueSize(int64(1)).Times(tripCount)
	queueMetrics.EXPECT().QueueSize(int64(-1)).Times(tripCount)
	metrics.EXPECT().StaleCheck(gomock.Any(), string(ResultQueued)).Times(tripCount)
	metrics.EXPECT().ScanRows(gomock.Any(), int64(tripCount)).Do(func(_ context.Context, rows int64) {
		scanDone <- rows
	})
	var queryCount int
	source.EXPECT().ListStaleBatch(gomock.Any(), time.Minute, gomock.Any(), batchSize).
		DoAndReturn(func(_ context.Context, _ time.Duration, after *model.StaleTrip, limit int) ([]model.StaleTrip, error) {
			queryCount++
			start := (queryCount - 1) * limit
			if queryCount == 1 {
				if after != nil {
					t.Fatalf("first batch cursor = %#v, want nil", after)
				}
			} else if after == nil || after.ID != trips[start-1].ID {
				t.Fatalf("batch %d cursor = %#v, want trip %q", queryCount, after, trips[start-1].ID)
			}
			end := min(start+limit, len(trips))
			return trips[start:end], nil
		}).Times(3)

	queue, err := NewQueue(tripCount, queueMetrics)
	if err != nil {
		t.Fatalf("NewQueue() error = %v", err)
	}
	scanner, err := NewScanner(
		source,
		queue,
		ticker,
		time.Minute,
		batchSize,
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
	select {
	case rows := <-scanDone:
		if rows != tripCount {
			t.Fatalf("ScanRows() = %d, want %d", rows, tripCount)
		}
	case <-time.After(time.Second):
		t.Fatal("scanner did not finish all batches")
	}
	if queryCount != 3 {
		t.Fatalf("ListStaleBatch() calls = %d, want 3", queryCount)
	}

	seen := make(map[string]bool, tripCount)
	for range tripCount {
		item, ok := queue.Take(context.Background())
		if !ok {
			t.Fatal("queue drained before all stale trips were returned")
		}
		seen[item.trip.ID] = true
		queue.Done(item)
	}
	if len(seen) != tripCount {
		t.Fatalf("queued %d unique trips, want %d", len(seen), tripCount)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scanner did not stop after context cancellation")
	}
	queue.Close()
}

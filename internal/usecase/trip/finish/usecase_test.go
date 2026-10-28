package finish

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/telemetry"
)

type mockMetrics struct {
	tripCompletedCalls int
}

func (m *mockMetrics) TripCompleted() {
	m.tripCompletedCalls++
}

func TestUsecaseDo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := &mockMetrics{}
		var logs bytes.Buffer
		now := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
		logger := telemetry.NewJSONLogger(slog.LevelInfo, "test", &logs)
		uc := New(repo, tx, func() time.Time { return now }, metrics, logger)

		activeTrip := &trip.Trip{
			ID:         "trip-123",
			UserID:     "user-1",
			DriverID:   "driver-1",
			Status:     trip.StatusActive,
			Price:      1500,
			StartPoint: trip.Point{Latitude: 1, Longitude: 2},
			EndPoint:   trip.Point{Latitude: 3, Longitude: 4},
			StartedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().GetByID(gomock.Any(), "trip-123").Return(activeTrip, nil)
		repo.EXPECT().Save(gomock.Any(), gomock.AssignableToTypeOf(&trip.Trip{})).DoAndReturn(func(_ context.Context, got *trip.Trip) error {
			if got.Status != trip.StatusCompleted {
				t.Fatalf("status = %q, want %q", got.Status, trip.StatusCompleted)
			}
			if got.FinishedAt == nil || !got.FinishedAt.Equal(now) {
				t.Fatalf("finished_at = %v, want %v", got.FinishedAt, now)
			}
			return nil
		})

		finished, err := uc.Do(context.Background(), "trip-123")
		if err != nil {
			t.Fatalf("Do() returned error: %v", err)
		}
		if finished == nil || finished.ID != "trip-123" {
			t.Fatalf("finished trip = %#v, want trip-123", finished)
		}
		if metrics.tripCompletedCalls != 1 {
			t.Errorf("TripCompleted() called %d times, want 1", metrics.tripCompletedCalls)
		}
		if !strings.Contains(logs.String(), `"event":"trip.completed"`) ||
			!strings.Contains(logs.String(), `"trip_id":"trip-123"`) ||
			strings.Contains(logs.String(), "user-1") || strings.Contains(logs.String(), "driver-1") {
			t.Errorf("business event log contains missing or sensitive fields: %s", logs.String())
		}
	})

	t.Run("driver busy error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := &mockMetrics{}
		uc := New(repo, tx, time.Now, metrics, telemetry.NewJSONLogger(slog.LevelInfo, "test", io.Discard))

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().GetByID(gomock.Any(), "trip-123").Return(nil, trip.ErrDriverBusy)

		_, err := uc.Do(context.Background(), "trip-123")
		if !errors.Is(err, trip.ErrDriverBusy) {
			t.Fatalf("Do() error = %v, want %v", err, trip.ErrDriverBusy)
		}
		if metrics.tripCompletedCalls != 0 {
			t.Errorf("TripCompleted() called %d times, want 0", metrics.tripCompletedCalls)
		}
	})

	t.Run("already completed trip error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := &mockMetrics{}
		uc := New(repo, tx, time.Now, metrics, telemetry.NewJSONLogger(slog.LevelInfo, "test", io.Discard))
		completedTrip := &trip.Trip{Status: trip.StatusCompleted}
		ctx := context.Background()

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().GetByID(gomock.Any(), "trip-456").Return(completedTrip, nil)

		_, err := uc.Do(ctx, "trip-456")
		if !errors.Is(err, trip.ErrTripCompleted) {
			t.Fatalf("Do() error = %v, want %v", err, trip.ErrTripCompleted)
		}
	})
}

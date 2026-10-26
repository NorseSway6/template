package finish

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"go.uber.org/mock/gomock"
)

func TestUsecaseDo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		now := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
		uc := New(repo, tx, func() time.Time { return now })

		activeTrip := &trip.Trip{
			ID:       "trip-123",
			UserID:   "user-1",
			DriverID: "driver-1",
			Status:   trip.StatusActive,
			Price:    1500,
			StartPoint: trip.Point{Latitude: 1, Longitude: 2},
			EndPoint:   trip.Point{Latitude: 3, Longitude: 4},
			StartedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
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
	})

	t.Run("already completed trip error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		uc := New(repo, tx, time.Now)
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

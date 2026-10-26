package create

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/model"
	"go.uber.org/mock/gomock"
)

func TestUsecaseDo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		uc := New(repo, tx, func() time.Time { return now }, func() string { return "trip-123" })
		in := model.CreateTripInput{UserID: "user-1", DriverID: "driver-1", Price: 1500, StartPoint: trip.Point{Latitude: 1, Longitude: 2}, EndPoint: trip.Point{Latitude: 3, Longitude: 4}}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&trip.Trip{})).DoAndReturn(func(_ context.Context, got *trip.Trip) error {
			if got.ID != "trip-123" {
				t.Fatalf("trip ID = %q, want %q", got.ID, "trip-123")
			}
			if got.Status != trip.StatusActive {
				t.Fatalf("status = %q, want %q", got.Status, trip.StatusActive)
			}
			return nil
		})

		got, err := uc.Do(context.Background(), in)
		if err != nil {
			t.Fatalf("Do() returned error: %v", err)
		}
		if got == nil {
			t.Fatal("Do() returned nil trip")
		}
		if got.Price != in.Price {
			t.Fatalf("price = %d, want %d", got.Price, in.Price)
		}
	})

	t.Run("repository error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		uc := New(repo, tx, time.Now, func() string { return "trip-123" })
		in := model.CreateTripInput{UserID: "user-1", DriverID: "driver-1", Price: 1500, StartPoint: trip.Point{Latitude: 1, Longitude: 2}, EndPoint: trip.Point{Latitude: 3, Longitude: 4}}
		expectedErr := errors.New("db failed")

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(expectedErr)

		_, err := uc.Do(context.Background(), in)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Do() error = %v, want %v", err, expectedErr)
		}
	})
}

package create

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/model"
)

func TestUsecaseDo(t *testing.T) {
	now := time.Date(2026, time.November, 15, 10, 0, 0, 0, time.UTC)
	input := model.CreatePositionInput{
		TripID:       "33333333-3333-3333-3333-333333333333",
		CurrentPoint: trip.Point{Latitude: 59.93, Longitude: 30.33},
		RecordedAt:   now,
	}

	t.Run("rejects invalid position before transaction", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := NewMockmetrics(ctrl)
		uc := New(repo, tx, func() time.Time { return now }, metrics)

		invalid := input
		invalid.CurrentPoint.Latitude = 91
		position, err := uc.Do(context.Background(), invalid)
		if position != nil {
			t.Fatalf("Do() position = %#v, want nil", position)
		}
		var validationErr *trip.ValidationError
		if !errors.As(err, &validationErr) || validationErr.Field != "current_point" {
			t.Fatalf("Do() error = %v, want current_point validation error", err)
		}
	})

	t.Run("returns trip lookup error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := NewMockmetrics(ctrl)
		lookupErr := errors.New("lookup failed")
		repo.EXPECT().GetByID(gomock.Any(), input.TripID).Return(nil, lookupErr)
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		)
		uc := New(repo, tx, func() time.Time { return now }, metrics)

		position, err := uc.Do(context.Background(), input)
		if position != nil || !errors.Is(err, lookupErr) {
			t.Fatalf("Do() = (%#v, %v), want nil and %v", position, err, lookupErr)
		}
	})

	t.Run("rejects position for completed trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := NewMockmetrics(ctrl)
		repo.EXPECT().GetByID(gomock.Any(), input.TripID).
			Return(&trip.Trip{ID: input.TripID, Status: trip.StatusCompleted}, nil)
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		)
		uc := New(repo, tx, func() time.Time { return now }, metrics)

		position, err := uc.Do(context.Background(), input)
		if position != nil || !errors.Is(err, trip.ErrTripCompleted) {
			t.Fatalf("Do() = (%#v, %v), want nil and %v", position, err, trip.ErrTripCompleted)
		}
	})

	t.Run("returns position save error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := NewMockmetrics(ctrl)
		saveErr := errors.New("save failed")
		repo.EXPECT().GetByID(gomock.Any(), input.TripID).
			Return(&trip.Trip{ID: input.TripID, Status: trip.StatusActive}, nil)
		repo.EXPECT().SavePosition(gomock.Any(), gomock.Any()).Return(saveErr)
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		)
		uc := New(repo, tx, func() time.Time { return now }, metrics)

		position, err := uc.Do(context.Background(), input)
		if position != nil || !errors.Is(err, saveErr) {
			t.Fatalf("Do() = (%#v, %v), want nil and %v", position, err, saveErr)
		}
	})

	t.Run("returns transaction error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := NewMockmetrics(ctrl)
		txErr := errors.New("transaction failed")
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).Return(txErr)
		uc := New(repo, tx, func() time.Time { return now }, metrics)

		position, err := uc.Do(context.Background(), input)
		if position != nil || !errors.Is(err, txErr) {
			t.Fatalf("Do() = (%#v, %v), want nil and %v", position, err, txErr)
		}
	})

	t.Run("saves position and records metric", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := NewMockmetrics(ctrl)
		repo.EXPECT().GetByID(gomock.Any(), input.TripID).
			Return(&trip.Trip{ID: input.TripID, Status: trip.StatusActive}, nil)
		repo.EXPECT().SavePosition(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, position *trip.Position) error {
				if position.TripID != input.TripID || position.CurPoint != input.CurrentPoint {
					t.Errorf("SavePosition() = %#v, want trip %q and point %+v",
						position, input.TripID, input.CurrentPoint)
				}
				return nil
			},
		)
		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		)
		metrics.EXPECT().PositionSaved(gomock.Any())
		uc := New(repo, tx, func() time.Time { return now }, metrics)

		position, err := uc.Do(context.Background(), input)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if position == nil || position.TripID != input.TripID {
			t.Fatalf("Do() position = %#v, want saved position for trip %q", position, input.TripID)
		}
	})
}

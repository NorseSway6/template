package get

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

func TestUsecaseDo(t *testing.T) {
	const tripID = "33333333-3333-3333-3333-333333333333"

	t.Run("returns trip lookup error without listing positions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		lookupErr := errors.New("trip lookup failed")
		repo.EXPECT().GetByID(gomock.Any(), tripID).Return(nil, lookupErr)
		uc := New(repo)

		positions, err := uc.Do(context.Background(), tripID)
		if positions != nil || !errors.Is(err, lookupErr) {
			t.Fatalf("Do() = (%#v, %v), want nil and %v", positions, err, lookupErr)
		}
	})

	t.Run("lists positions after finding trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		expectedTrip := &trip.Trip{ID: tripID, Status: trip.StatusActive}
		expectedPositions := []*trip.Position{{TripID: tripID}}
		repo.EXPECT().GetByID(gomock.Any(), tripID).Return(expectedTrip, nil)
		repo.EXPECT().ListPositions(gomock.Any(), tripID).Return(expectedPositions, nil)
		uc := New(repo)

		positions, err := uc.Do(context.Background(), tripID)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if len(positions) != 1 || positions[0] != expectedPositions[0] {
			t.Fatalf("Do() positions = %#v, want %#v", positions, expectedPositions)
		}
	})

	t.Run("returns position listing error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		listErr := errors.New("position listing failed")
		repo.EXPECT().GetByID(gomock.Any(), tripID).
			Return(&trip.Trip{ID: tripID, Status: trip.StatusActive}, nil)
		repo.EXPECT().ListPositions(gomock.Any(), tripID).Return(nil, listErr)
		uc := New(repo)

		positions, err := uc.Do(context.Background(), tripID)
		if positions != nil || !errors.Is(err, listErr) {
			t.Fatalf("Do() = (%#v, %v), want nil and %v", positions, err, listErr)
		}
	})
}

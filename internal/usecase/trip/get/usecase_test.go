package get

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

func TestUsecaseDo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		uc := New(repo)
		expected := &trip.Trip{ID: "trip-42", Status: trip.StatusActive, StartedAt: time.Now()}
		repo.EXPECT().GetByID(gomock.Any(), "trip-42").Return(expected, nil)

		got, err := uc.Do(context.Background(), "trip-42")
		if err != nil {
			t.Fatalf("Do() returned error: %v", err)
		}
		if got != expected {
			t.Fatal("Do() returned unexpected trip")
		}
	})

	t.Run("not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		uc := New(repo)
		expectedErr := trip.ErrTripNotFound
		repo.EXPECT().GetByID(gomock.Any(), "missing").Return(nil, expectedErr)

		_, err := uc.Do(context.Background(), "missing")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Do() error = %v, want %v", err, expectedErr)
		}
	})
}

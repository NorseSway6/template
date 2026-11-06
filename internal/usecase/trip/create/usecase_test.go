package create

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/telemetry"
	"github.com/NorseSway6/template.git/internal/model"
)

const testTripID = "33333333-3333-3333-3333-333333333333"

func TestUsecaseDo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := NewMockmetrics(ctrl)
		var logs bytes.Buffer
		now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		logger := telemetry.NewJSONLogger(slog.LevelInfo, "test", &logs)
		uc := New(repo, tx, func() time.Time { return now }, func() string { return testTripID }, metrics, logger)
		in := model.CreateTripInput{UserID: "11111111-1111-1111-1111-111111111111", DriverID: "22222222-2222-2222-2222-222222222222", Price: 1500, StartPoint: trip.Point{Latitude: 1, Longitude: 2}, EndPoint: trip.Point{Latitude: 3, Longitude: 4}}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().Create(gomock.Any(), gomock.AssignableToTypeOf(&trip.Trip{})).DoAndReturn(func(_ context.Context, got *trip.Trip) error {
			if got.ID != testTripID {
				t.Fatalf("trip ID = %q, want %q", got.ID, testTripID)
			}
			if got.Status != trip.StatusActive {
				t.Fatalf("status = %q, want %q", got.Status, trip.StatusActive)
			}
			return nil
		})
		metrics.EXPECT().TripCreated()

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
		if !strings.Contains(logs.String(), `"event":"trip.created"`) ||
			!strings.Contains(logs.String(), `"trip_id":"`+testTripID+`"`) ||
			strings.Contains(logs.String(), "11111111-1111-1111-1111-111111111111") ||
			strings.Contains(logs.String(), "22222222-2222-2222-2222-222222222222") {
			t.Errorf("business event log contains missing or sensitive fields: %s", logs.String())
		}
	})

	t.Run("driver busy error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := NewMockmetrics(ctrl)
		uc := New(repo, tx, time.Now, func() string { return testTripID }, metrics, telemetry.NewJSONLogger(slog.LevelInfo, "test", io.Discard))
		in := model.CreateTripInput{UserID: "11111111-1111-1111-1111-111111111111", DriverID: "22222222-2222-2222-2222-222222222222", Price: 1500, StartPoint: trip.Point{Latitude: 1, Longitude: 2}, EndPoint: trip.Point{Latitude: 3, Longitude: 4}}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(trip.ErrDriverBusy)
		metrics.EXPECT().TripCreated().Times(0)

		_, err := uc.Do(context.Background(), in)
		if err != trip.ErrDriverBusy {
			t.Fatalf("Do() error = %v, want %v", err, trip.ErrDriverBusy)
		}
	})
}

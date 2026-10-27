package create

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/telemetry"
	"github.com/NorseSway6/template.git/internal/model"
	"go.uber.org/mock/gomock"
)

type mockMetrics struct {
	tripCreatedCalls int
}

func (m *mockMetrics) TripCreated() {
	m.tripCreatedCalls++
}

func TestUsecaseDo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := &mockMetrics{}
		var logs bytes.Buffer
		now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		logger := telemetry.NewJSONLogger(slog.LevelInfo, "test", &logs)
		uc := New(repo, tx, func() time.Time { return now }, func() string { return "trip-123" }, metrics, logger)
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
		if metrics.tripCreatedCalls != 1 {
			t.Errorf("TripCreated() called %d times, want 1", metrics.tripCreatedCalls)
		}
		if !strings.Contains(logs.String(), `"event":"trip.created"`) ||
			!strings.Contains(logs.String(), `"trip_id":"trip-123"`) ||
			strings.Contains(logs.String(), "user-1") || strings.Contains(logs.String(), "driver-1") {
			t.Errorf("business event log contains missing or sensitive fields: %s", logs.String())
		}
	})

	t.Run("driver busy error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		metrics := &mockMetrics{}
		uc := New(repo, tx, time.Now, func() string { return "trip-123" }, metrics, telemetry.NewJSONLogger(slog.LevelInfo, "test", io.Discard))
		in := model.CreateTripInput{UserID: "user-1", DriverID: "driver-1", Price: 1500, StartPoint: trip.Point{Latitude: 1, Longitude: 2}, EndPoint: trip.Point{Latitude: 3, Longitude: 4}}

		tx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(trip.ErrDriverBusy)

		_, err := uc.Do(context.Background(), in)
		if err != trip.ErrDriverBusy {
			t.Fatalf("Do() error = %v, want %v", err, trip.ErrDriverBusy)
		}
		if metrics.tripCreatedCalls != 0 {
			t.Errorf("TripCreated() called %d times, want 0", metrics.tripCreatedCalls)
		}
	})
}

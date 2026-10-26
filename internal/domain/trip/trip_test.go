package trip

import (
	"testing"
	"time"
)

func TestTripNew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		got, err := New("trip-1", "user-1", "driver-1", 1500, Point{Latitude: 55.75, Longitude: 37.62}, Point{Latitude: 55.76, Longitude: 37.61}, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))
		if err != nil {
			t.Fatalf("New() returned error: %v", err)
		}
		if got.Status != StatusActive {
			t.Fatalf("status = %q, want %q", got.Status, StatusActive)
		}
	})

	tests := []struct {
		name    string
		price   int64
		start   Point
		end     Point
		wantErr bool
	}{
		{name: "negative price", price: -1, start: Point{Latitude: 0, Longitude: 0}, end: Point{Latitude: 1, Longitude: 1}, wantErr: true},
		{name: "invalid start latitude", price: 100, start: Point{Latitude: 91, Longitude: 0}, end: Point{Latitude: 1, Longitude: 1}, wantErr: true},
		{name: "invalid end longitude", price: 100, start: Point{Latitude: 0, Longitude: 0}, end: Point{Latitude: 1, Longitude: 181}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New("trip-1", "user-1", "driver-1", tt.price, tt.start, tt.end, time.Now())
			if tt.wantErr && err == nil {
				t.Fatal("New() error = nil, want validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}
		})
	}
}

func TestTripFinish(t *testing.T) {
	t.Run("finish active trip", func(t *testing.T) {
		trip := &Trip{Status: StatusActive}
		finishedAt := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)

		if err := trip.Finish(finishedAt); err != nil {
			t.Fatalf("Finish() returned error: %v", err)
		}
		if trip.Status != StatusCompleted {
			t.Fatalf("status = %q, want %q", trip.Status, StatusCompleted)
		}
		if trip.FinishedAt == nil || !trip.FinishedAt.Equal(finishedAt) {
			t.Fatalf("finished_at = %v, want %v", trip.FinishedAt, finishedAt)
		}
	})

	t.Run("finish completed trip returns ErrTripCompleted", func(t *testing.T) {
		trip := &Trip{Status: StatusCompleted}
		err := trip.Finish(time.Now())
		if err != ErrTripCompleted {
			t.Fatalf("Finish() error = %v, want %v", err, ErrTripCompleted)
		}
	})
}

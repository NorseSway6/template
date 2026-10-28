package trip

import (
	"testing"
	"time"
)

func TestTripNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		price      int64
		start      Point
		end        Point
		wantErr    bool
		wantField  string
		wantReason string
	}{
		{
			name:  "valid trip",
			price: 1500,
			start: Point{Latitude: 55.75, Longitude: 37.62},
			end:   Point{Latitude: 55.76, Longitude: 37.61},
		},
		{
			name:  "zero price is valid",
			price: 0,
			start: Point{Latitude: 0, Longitude: 0},
			end:   Point{Latitude: 1, Longitude: 1},
		},
		{
			name:  "minimum latitude is valid",
			price: 100,
			start: Point{Latitude: -90},
			end:   Point{},
		},
		{
			name:  "maximum latitude is valid",
			price: 100,
			start: Point{},
			end:   Point{Latitude: 90},
		},
		{
			name:  "minimum longitude is valid",
			price: 100,
			start: Point{Longitude: -180},
			end:   Point{},
		},
		{
			name:  "maximum longitude is valid",
			price: 100,
			start: Point{},
			end:   Point{Longitude: 180},
		},
		{
			name:       "negative price",
			price:      -1,
			start:      Point{},
			end:        Point{Latitude: 1, Longitude: 1},
			wantErr:    true,
			wantField:  "price",
			wantReason: "must be >= 0",
		},
		{
			name:       "invalid start latitude",
			price:      100,
			start:      Point{Latitude: 91},
			end:        Point{Latitude: 1, Longitude: 1},
			wantErr:    true,
			wantField:  "start_point",
			wantReason: "coordinates out of bounds",
		},
		{
			name:       "invalid end longitude",
			price:      100,
			start:      Point{},
			end:        Point{Latitude: 1, Longitude: 181},
			wantErr:    true,
			wantField:  "end_point",
			wantReason: "coordinates out of bounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			startedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
			got, err := New("trip-1", "user-1", "driver-1", tt.price, tt.start, tt.end, startedAt)
			if tt.wantErr {
				if got != nil {
					t.Fatalf("New() trip = %#v, want nil", got)
				}
				validationErr, ok := err.(*ValidationError)
				if !ok {
					t.Fatalf("New() error = %v, want *ValidationError", err)
				}
				if validationErr.Field != tt.wantField || validationErr.Reason != tt.wantReason {
					t.Fatalf("validation error = %#v, want field=%q reason=%q", validationErr, tt.wantField, tt.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() returned error: %v", err)
			}
			if got == nil {
				t.Fatal("New() returned nil trip")
			}
			if got.ID != "trip-1" || got.UserID != "user-1" || got.DriverID != "driver-1" {
				t.Fatalf("identity fields = (%q, %q, %q), want trip-1, user-1, driver-1", got.ID, got.UserID, got.DriverID)
			}
			if got.Status != StatusActive || got.Price != tt.price {
				t.Fatalf("status/price = (%q, %d), want (%q, %d)", got.Status, got.Price, StatusActive, tt.price)
			}
			if got.StartPoint != tt.start || got.EndPoint != tt.end {
				t.Fatalf("points = (%+v, %+v), want (%+v, %+v)", got.StartPoint, got.EndPoint, tt.start, tt.end)
			}
			if !got.StartedAt.Equal(startedAt) {
				t.Fatalf("started_at = %v, want %v", got.StartedAt, startedAt)
			}
		})
	}
}

func TestValidationErrorError(t *testing.T) {
	err := &ValidationError{Field: "price", Reason: "must be >= 0"}
	if got, want := err.Error(), "price: must be >= 0"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
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

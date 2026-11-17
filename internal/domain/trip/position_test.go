package trip

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewPosition(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.November, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		tripID     string
		point      Point
		recordedAt time.Time
		wantField  string
		wantReason string
	}{
		{
			name:       "valid position",
			tripID:     testTripID,
			point:      Point{Latitude: 59.93, Longitude: 30.33},
			recordedAt: now,
		},
		{
			name:       "rejects malformed trip id",
			tripID:     "not-a-uuid",
			point:      Point{},
			recordedAt: now,
			wantField:  "trip_id",
			wantReason: "must be a non-zero UUID",
		},
		{
			name:       "rejects zero trip id",
			tripID:     uuid.Nil.String(),
			point:      Point{},
			recordedAt: now,
			wantField:  "trip_id",
			wantReason: "must be a non-zero UUID",
		},
		{
			name:       "rejects invalid current point",
			tripID:     testTripID,
			point:      Point{Latitude: 91},
			recordedAt: now,
			wantField:  "current_point",
			wantReason: coordinateBoundsReason,
		},
		{
			name:       "rejects timestamp too far in future",
			tripID:     testTripID,
			point:      Point{},
			recordedAt: now.Add(time.Minute + time.Nanosecond),
			wantField:  "recorded_at",
			wantReason: "is in the future",
		},
		{
			name:       "allows timestamp within one minute in future",
			tripID:     testTripID,
			point:      Point{},
			recordedAt: now.Add(time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			position, err := NewPosition(tt.tripID, tt.point, tt.recordedAt, now)
			if tt.wantField != "" {
				if position != nil {
					t.Fatalf("NewPosition() = %#v, want nil", position)
				}
				validationErr, ok := err.(*ValidationError)
				if !ok {
					t.Fatalf("NewPosition() error = %v, want *ValidationError", err)
				}
				if validationErr.Field != tt.wantField || validationErr.Reason != tt.wantReason {
					t.Fatalf("validation error = %#v, want field=%q reason=%q", validationErr, tt.wantField, tt.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewPosition() error = %v", err)
			}
			if position == nil {
				t.Fatal("NewPosition() returned nil position")
			}
			if position.TripID != tt.tripID || position.CurPoint != tt.point {
				t.Fatalf("position identity/point = (%q, %+v), want (%q, %+v)",
					position.TripID, position.CurPoint, tt.tripID, tt.point)
			}
			if !position.RecordedAt.Equal(tt.recordedAt) || !position.CreatedAt.Equal(now) {
				t.Fatalf("position timestamps = (%v, %v), want (%v, %v)",
					position.RecordedAt, position.CreatedAt, tt.recordedAt, now)
			}
		})
	}
}

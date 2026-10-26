package domainerror

import (
	"net/http"
	"testing"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantCodeStatus int
	}{
		{name: "validation", err: &trip.ValidationError{Field: "body", Reason: "invalid JSON"}, wantCode: "invalid_request", wantCodeStatus: http.StatusBadRequest},
		{name: "trip not found", err: trip.ErrTripNotFound, wantCode: "trip_not_found", wantCodeStatus: http.StatusNotFound},
		{name: "trip completed", err: trip.ErrTripCompleted, wantCode: "trip_completed", wantCodeStatus: http.StatusConflict},
		{name: "driver busy", err: trip.ErrDriverBusy, wantCode: "driver_busy", wantCodeStatus: http.StatusConflict},
		{name: "default", err: assertErr("boom"), wantCode: "internal_error", wantCodeStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := Problem(tt.err)
			if status != tt.wantCodeStatus {
				t.Fatalf("status = %d, want %d", status, tt.wantCodeStatus)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", body.Code, tt.wantCode)
			}
		})
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

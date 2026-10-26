package finish

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"go.uber.org/mock/gomock"
)

func TestHandlerFinish(t *testing.T) {
	tripID := api.TripId(uuid.MustParse("33333333-3333-3333-3333-333333333333"))
	userID := "11111111-1111-1111-1111-111111111111"
	driverID := "22222222-2222-2222-2222-222222222222"

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		finishedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		resTrip := &trip.Trip{
			ID:         tripID.String(),
			UserID:     userID,
			DriverID:   driverID,
			Status:     trip.StatusCompleted,
			Price:      1500,
			StartedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			FinishedAt: &finishedAt,
		}
		uc.EXPECT().Do(gomock.Any(), tripID.String()).Return(resTrip, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		res := httptest.NewRecorder()
		h.Finish(res, req, tripID)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if !strings.Contains(res.Body.String(), `"status":"completed"`) {
			t.Fatalf("response body = %s", res.Body.String())
		}
	})

	t.Run("trip already completed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		uc.EXPECT().Do(gomock.Any(), tripID.String()).Return(nil, trip.ErrTripCompleted)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		res := httptest.NewRecorder()
		h.Finish(res, req, tripID)
		if res.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusConflict)
		}
		if !strings.Contains(res.Body.String(), `"code":"trip_completed"`) {
			t.Fatalf("response body = %s", res.Body.String())
		}
	})

	t.Run("repository error hides details", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		uc.EXPECT().Do(gomock.Any(), tripID.String()).Return(nil, errors.New("db failure details"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		res := httptest.NewRecorder()
		h.Finish(res, req, tripID)
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
		}
		if strings.Contains(res.Body.String(), "db failure details") {
			t.Fatalf("response body unexpectedly includes internal error details: %s", res.Body.String())
		}
	})
}

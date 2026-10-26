package get

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

func TestHandlerGet(t *testing.T) {
	tripID := api.TripId(uuid.MustParse("33333333-3333-3333-3333-333333333333"))
	userID := "11111111-1111-1111-1111-111111111111"
	driverID := "22222222-2222-2222-2222-222222222222"

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		tripRec := &trip.Trip{
			ID:         tripID.String(),
			UserID:     userID,
			DriverID:   driverID,
			Status:     trip.StatusActive,
			Price:      1500,
			StartPoint: trip.Point{Latitude: 10, Longitude: 20},
			EndPoint:   trip.Point{Latitude: 30, Longitude: 40},
			StartedAt:  time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		}
		uc.EXPECT().Do(gomock.Any(), tripID.String()).Return(tripRec, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String(), nil)
		res := httptest.NewRecorder()
		h.Get(res, req, tripID)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
		}
		if !strings.Contains(res.Body.String(), `"id":"33333333-3333-3333-3333-333333333333"`) {
			t.Fatalf("response body = %s", res.Body.String())
		}
	})

	t.Run("trip not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		missingID := "00000000-0000-0000-0000-000000000001"
		uc.EXPECT().Do(gomock.Any(), missingID).Return(nil, trip.ErrTripNotFound)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+missingID, nil)
		res := httptest.NewRecorder()
		h.Get(res, req, api.TripId(uuid.MustParse(missingID)))
		if res.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
		}
		if !strings.Contains(res.Body.String(), `"code":"trip_not_found"`) {
			t.Fatalf("response body = %s", res.Body.String())
		}
	})

	t.Run("repository error hides details", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		uc.EXPECT().Do(gomock.Any(), tripID.String()).Return(nil, errors.New("db failure details"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String(), nil)
		res := httptest.NewRecorder()
		h.Get(res, req, tripID)
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
		}
		if strings.Contains(res.Body.String(), "db failure details") {
			t.Fatalf("response body unexpectedly includes internal error details: %s", res.Body.String())
		}
	})
}

package create

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/model"
	"go.uber.org/mock/gomock"
)

func TestHandlerCreate(t *testing.T) {
	validBody := `{"user_id":"11111111-1111-1111-1111-111111111111","driver_id":"22222222-2222-2222-2222-222222222222","start_point":{"latitude":10,"longitude":20},"end_point":{"latitude":30,"longitude":40},"price":1500}`
	validTrip := &trip.Trip{
		ID:       "33333333-3333-3333-3333-333333333333",
		UserID:   "11111111-1111-1111-1111-111111111111",
		DriverID: "22222222-2222-2222-2222-222222222222",
		Status:   trip.StatusActive,
		Price:    1500,
		StartPoint: trip.Point{Latitude: 10, Longitude: 20},
		EndPoint:   trip.Point{Latitude: 30, Longitude: 40},
		StartedAt: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		uc.EXPECT().Do(gomock.Any(), gomock.AssignableToTypeOf(model.CreateTripInput{})).Return(validTrip, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody))
		res := httptest.NewRecorder()
		h.Create(res, req, api.CreateTripParams{})
		if res.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
		}
		if got := res.Header().Get("Location"); got != "/api/v1/trips/33333333-3333-3333-3333-333333333333" {
			t.Fatalf("Location = %q", got)
		}
		var body map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body["status"] != "active" {
			t.Fatalf("status = %v, want active", body["status"])
		}
	})

	t.Run("driver busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		uc.EXPECT().Do(gomock.Any(), gomock.Any()).Return(nil, trip.ErrDriverBusy)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody))
		res := httptest.NewRecorder()
		h.Create(res, req, api.CreateTripParams{})
		if res.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusConflict)
		}
		if !strings.Contains(res.Body.String(), `"code":"driver_busy"`) {
			t.Fatalf("response body = %s", res.Body.String())
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader("{not-json"))
		res := httptest.NewRecorder()
		h.Create(res, req, api.CreateTripParams{})
		if res.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
		}
		if !strings.Contains(res.Body.String(), `"code":"invalid_request"`) {
			t.Fatalf("response body = %s", res.Body.String())
		}
	})

	t.Run("repository error hides details", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		uc := NewMockusecase(ctrl)
		h := New(uc)
		uc.EXPECT().Do(gomock.Any(), gomock.Any()).Return(nil, errors.New("db failure details"))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody))
		res := httptest.NewRecorder()
		h.Create(res, req, api.CreateTripParams{})
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", res.Code, http.StatusInternalServerError)
		}
		body := res.Body.String()
		if strings.Contains(body, "db failure details") {
			t.Fatalf("response body unexpectedly contains internal error details: %s", body)
		}
		if !strings.Contains(body, `"code":"internal_error"`) {
			t.Fatalf("response body = %s", body)
		}
	})
}

var _ = context.Background

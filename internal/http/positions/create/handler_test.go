package create

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/model"
)

func TestCreatePositionCancelsQueuedPushAfterSuccessfulSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripID := uuid.NewString()
	uc := NewMockusecase(ctrl)
	position := &trip.Position{
		ID:         "1",
		CurPoint:   trip.Point{Latitude: 59.93, Longitude: 30.33},
		RecordedAt: time.Now().UTC(),
	}

	uc.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, in model.CreatePositionInput) (*trip.Position, error) {
			got := *position
			got.TripID = in.TripID
			return &got, nil
		},
	)
	canceller := NewMockqueueCanceller(ctrl)
	canceller.EXPECT().Cancel(tripID)
	handler := New(uc, canceller)
	body := `{"coordinates":{"latitude":59.93,"longitude":30.33},"recorded_at":"2026-11-15T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID+"/positions", strings.NewReader(body))
	res := httptest.NewRecorder()

	handler.CreatePosition(res, req, uuid.MustParse(tripID))

	if res.Code != http.StatusCreated {
		t.Fatalf("CreatePosition() status = %d, want %d; body: %s", res.Code, http.StatusCreated, res.Body.String())
	}
}

func TestCreatePositionRejectsIncorrectFieldCasing(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "incorrect coordinates casing",
			body: `{"Coordinates":{"latitude":59.93,"longitude":30.33},"recorded_at":"2026-11-15T00:00:00Z"}`,
		},
		{
			name: "incorrect latitude casing",
			body: `{"coordinates":{"Latitude":59.93,"longitude":30.33},"recorded_at":"2026-11-15T00:00:00Z"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			handler := New(NewMockusecase(ctrl), NewMockqueueCanceller(ctrl))
			tripID := uuid.New()
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/trips/"+tripID.String()+"/positions",
				strings.NewReader(tt.body),
			)
			res := httptest.NewRecorder()

			handler.CreatePosition(res, req, tripID)

			if res.Code != http.StatusBadRequest {
				t.Fatalf("CreatePosition() status = %d, want %d; body: %s",
					res.Code, http.StatusBadRequest, res.Body.String())
			}
			var problem api.Problem
			if err := json.Unmarshal(res.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem response: %v", err)
			}
			if problem.Code != "invalid_request" {
				t.Fatalf("problem code = %q, want %q", problem.Code, "invalid_request")
			}
		})
	}
}

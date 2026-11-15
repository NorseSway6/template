package create

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/http/domainerror"
)

// Handler обрабатывает HTTP запросы для создания поездки.
type Handler struct {
	uc usecase
}

type createTripRequest struct {
	UserID     *api.TripId       `json:"user_id"`
	DriverID   *api.TripId       `json:"driver_id"`
	StartPoint *coordinatesInput `json:"start_point"`
	EndPoint   *coordinatesInput `json:"end_point"`
	Price      *int64            `json:"price"`
}

type coordinatesInput struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// New создает новый Handler.
func New(uc usecase) *Handler {
	return &Handler{uc: uc}
}

// Create создает поездку.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	req, err := decodeRequest(r)
	if err != nil {
		domainerror.WriteError(w, r, &trip.ValidationError{Field: "body", Reason: "invalid JSON"})
		return
	}

	t, err := h.uc.Do(r.Context(), toInput(req))
	if err != nil {
		domainerror.WriteError(w, r, err)
		return
	}

	body, err := toAPI(t)
	if err != nil {
		domainerror.WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+t.ID)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(body)
}

func decodeRequest(r *http.Request) (api.CreateTripJSONRequestBody, error) {
	var input createTripRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return api.CreateTripJSONRequestBody{}, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return api.CreateTripJSONRequestBody{}, errors.New("request body contains multiple JSON values")
		}
		return api.CreateTripJSONRequestBody{}, err
	}

	if input.UserID == nil || input.DriverID == nil || input.StartPoint == nil ||
		input.EndPoint == nil || input.Price == nil ||
		input.StartPoint.Latitude == nil || input.StartPoint.Longitude == nil ||
		input.EndPoint.Latitude == nil || input.EndPoint.Longitude == nil {
		return api.CreateTripJSONRequestBody{}, errors.New("request body is missing required fields")
	}

	return api.CreateTripJSONRequestBody{
		UserId:   *input.UserID,
		DriverId: *input.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  *input.StartPoint.Latitude,
			Longitude: *input.StartPoint.Longitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  *input.EndPoint.Latitude,
			Longitude: *input.EndPoint.Longitude,
		},
		Price: *input.Price,
	}, nil
}

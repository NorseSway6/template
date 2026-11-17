package create

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/http/domainerror"
)

// Handler обрабатывает HTTP-запросы, связанные с созданием позиции.
type Handler struct {
	uc     usecase
	cancel queueCanceller
}

type createPositionRequest struct {
	Coordinates *coordinatesRequest `json:"coordinates"`
	RecordedAt  *time.Time          `json:"recorded_at"`
}

type coordinatesRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// New создает новый Handler.
func New(uc usecase, cancel queueCanceller) *Handler {
	return &Handler{uc: uc, cancel: cancel}
}

// CreatePosition обрабатывает HTTP-запрос на создание позиции.
func (h *Handler) CreatePosition(w http.ResponseWriter, r *http.Request, id api.TripId) {
	req, err := decodeRequest(r)
	if err != nil {
		domainerror.WriteError(w, r, &trip.ValidationError{Field: "body", Reason: "invalid JSON or missing required fields"})
		return
	}

	p, err := h.uc.Do(r.Context(), toInput(req, id.String()))
	if err != nil {
		domainerror.WriteError(w, r, err)
		return
	}
	h.cancel.Cancel(id.String())

	body, err := toAPI(p)
	if err != nil {
		domainerror.WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(body)
}

// decodeRequest декодирует тело запроса в структуру createPositionRequest.
func decodeRequest(r *http.Request) (api.CreateTripPositionJSONRequestBody, error) {
	var input createPositionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return api.CreateTripPositionJSONRequestBody{}, err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return api.CreateTripPositionJSONRequestBody{}, errors.New("request body contains multiple JSON values")
		}
		return api.CreateTripPositionJSONRequestBody{}, err
	}
	if input.Coordinates == nil || input.Coordinates.Latitude == nil ||
		input.Coordinates.Longitude == nil || input.RecordedAt == nil {
		return api.CreateTripPositionJSONRequestBody{}, errors.New("request body is missing required fields")
	}

	return api.CreateTripPositionJSONRequestBody{
		Coordinates: api.Coordinates{
			Latitude:  *input.Coordinates.Latitude,
			Longitude: *input.Coordinates.Longitude,
		},
		RecordedAt: *input.RecordedAt,
	}, nil
}

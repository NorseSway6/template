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

// decodeRequest декодирует тело запроса, проверяя точное написание полей.
func decodeRequest(r *http.Request) (api.CreateTripPositionJSONRequestBody, error) {
	var rawRequest map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&rawRequest); err != nil {
		return api.CreateTripPositionJSONRequestBody{}, err
	}

	for field := range rawRequest {
		if field != "coordinates" && field != "recorded_at" {
			return api.CreateTripPositionJSONRequestBody{}, errors.New("request body contains an unknown field")
		}
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return api.CreateTripPositionJSONRequestBody{}, errors.New("request body contains multiple JSON values")
		}
		return api.CreateTripPositionJSONRequestBody{}, err
	}

	coordinatesRaw, hasCoordinates := rawRequest["coordinates"]
	recordedAtRaw, hasRecordedAt := rawRequest["recorded_at"]
	if !hasCoordinates || !hasRecordedAt {
		return api.CreateTripPositionJSONRequestBody{}, errors.New("request body is missing required fields")
	}

	var rawCoordinates map[string]json.RawMessage
	if err := json.Unmarshal(coordinatesRaw, &rawCoordinates); err != nil {
		return api.CreateTripPositionJSONRequestBody{}, err
	}
	for field := range rawCoordinates {
		if field != "latitude" && field != "longitude" {
			return api.CreateTripPositionJSONRequestBody{}, errors.New("coordinates contains an unknown field")
		}
	}

	var coordinates coordinatesRequest
	if err := json.Unmarshal(coordinatesRaw, &coordinates); err != nil {
		return api.CreateTripPositionJSONRequestBody{}, err
	}
	var recordedAt *time.Time
	if err := json.Unmarshal(recordedAtRaw, &recordedAt); err != nil {
		return api.CreateTripPositionJSONRequestBody{}, err
	}
	if coordinates.Latitude == nil || coordinates.Longitude == nil || recordedAt == nil {
		return api.CreateTripPositionJSONRequestBody{}, errors.New("request body is missing required fields")
	}

	return api.CreateTripPositionJSONRequestBody{
		Coordinates: api.Coordinates{
			Latitude:  *coordinates.Latitude,
			Longitude: *coordinates.Longitude,
		},
		RecordedAt: *recordedAt,
	}, nil
}

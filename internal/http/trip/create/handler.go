package create

import (
	"encoding/json"
	"net/http"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/http/domainerror"
)

// Handler обрабатывает HTTP запросы для создания поездки.
type Handler struct {
	uc usecase
}

// New создает новый Handler.
func New(uc usecase) *Handler {
	return &Handler{uc: uc}
}

// Create создает поездку.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	var req api.CreateTripJSONRequestBody

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

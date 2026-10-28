package finish

import (
	"encoding/json"
	"net/http"

	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/http/domainerror"
)

// Handler обрабатывает HTTP запросы для завершения поездки.
type Handler struct {
	uc usecase
}

// New создает новый Handler.
func New(uc usecase) *Handler {
	return &Handler{uc: uc}
}

// Finish завершает поездку.
func (h *Handler) Finish(w http.ResponseWriter, r *http.Request, id api.TripId) {
	t, err := h.uc.Do(r.Context(), id.String())
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
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

package get

import (
	"encoding/json"
	"net/http"

	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/http/domainerror"
)

// Handler обрабатывает HTTP-запросы для получения маршрута поездки.
type Handler struct {
	uc usecase
}

// New создает новый Handler.
func New(uc usecase) *Handler {
	return &Handler{uc: uc}
}

// GetPosition возвращает маршрут поездки.
func (h *Handler) GetPosition(w http.ResponseWriter, r *http.Request, id api.TripId) {
	positions, err := h.uc.Do(r.Context(), id.String())
	if err != nil {
		domainerror.WriteError(w, r, err)
		return
	}

	body, err := toAPI(positions)
	if err != nil {
		domainerror.WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

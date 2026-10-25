package domainerror

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
)

// Problem преобразует доменну ошибку в Problem.
func Problem(err error) (status int, body api.Problem) {
	var verr *trip.ValidationError

	switch {
	case errors.As(err, &verr):
		return problem(http.StatusBadRequest, "invalid_request", "Invalid request", verr.Error())
	case errors.Is(err, trip.ErrTripNotFound):
		return problem(http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found")
	case errors.Is(err, trip.ErrTripCompleted):
		return problem(http.StatusConflict, "trip_completed", "Trip completed", "Operation is not allowed for a completed trip")
	case errors.Is(err, trip.ErrDriverBusy):
		return problem(http.StatusConflict, "driver_busy", "Driver busy", "Driver already has an active trip")
	case errors.Is(err, trip.ErrIdempotencyConflict):
		return problem(http.StatusConflict, "idempotency_conflict", "Idempotency conflict", "Idempotency-Key was already used with a different request body")
	default:
		return problem(http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
	}
}

// WriteError записывает ошибку в ответ.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status, body := Problem(err)
	body.Instance = &r.URL.Path

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// problem создает Problem.
func problem(status int, code, title, detail string) (int, api.Problem) {
	return status, api.Problem{
		Type:   code,
		Title:  title,
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}
}

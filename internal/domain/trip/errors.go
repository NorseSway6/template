package trip

import "errors"

var (
	ErrDriverBusy          = errors.New("driver is busy")
	ErrTripNotFound        = errors.New("trip not found")
	ErrTripCompleted       = errors.New("trip already completed")
	ErrIdempotencyConflict = errors.New("idempotency conflict: same key with different body")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Reason
}

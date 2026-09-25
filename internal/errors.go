package internal

import "errors"

var (
	ErrDriverBusy = errors.New("driver is busy")
	ErrTripNotFound  = errors.New("trip not found")
	ErrTripCompleted = errors.New("trip already completed")
)

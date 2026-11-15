package model

import (
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

// StaleTrip предстваляет поездку, по которой давно не было координат
type StaleTrip struct {
	ID         string
	DriverID   string
	StaleSince time.Time
}

// Position представляет собой структуру, содержащую информацию о позиции поездки.
type CreatePositionInput struct {
	TripID       string
	CurrentPoint trip.Point
	RecordedAt   time.Time
}

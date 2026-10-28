package model

import (
	"github.com/NorseSway6/template.git/internal/domain/trip"
)

// CreateTripInput представляет входные данные для создания поездки.
type CreateTripInput struct {
	UserID     string
	DriverID   string
	StartPoint trip.Point
	EndPoint   trip.Point
	Price      int64
}

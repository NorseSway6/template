package get

import (
	"fmt"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/google/uuid"
)

func toAPI(t *trip.Trip) (api.Trip, error) {
	id, err := uuid.Parse(t.ID)
	if err != nil {
		return api.Trip{}, fmt.Errorf("parse trip id: %w", err)
	}
	userID, err := uuid.Parse(t.UserID)
	if err != nil {
		return api.Trip{}, fmt.Errorf("parse user id: %w", err)
	}
	driverID, err := uuid.Parse(t.DriverID)
	if err != nil {
		return api.Trip{}, fmt.Errorf("parse driver id: %w", err)
	}

	return api.Trip{
		Id:       id,
		UserId:   userID,
		DriverId: driverID,
		StartPoint: api.Coordinates{
			Latitude:  t.StartPoint.Latitude,
			Longitude: t.StartPoint.Longitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  t.EndPoint.Latitude,
			Longitude: t.EndPoint.Longitude,
		},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}, nil
}

package create

import (
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/model"
)

func toInput(req api.CreateTripPositionJSONRequestBody, tripID string) model.CreatePositionInput {
	return model.CreatePositionInput{
		TripID:       tripID,
		CurrentPoint: trip.Point{Latitude: req.Coordinates.Latitude, Longitude: req.Coordinates.Longitude},
		RecordedAt:   req.RecordedAt,
	}
}

func toAPI(p *trip.Position) (api.TripPosition, error) {
	id, err := strconv.ParseInt(p.ID, 10, 64)
	if err != nil {
		return api.TripPosition{}, fmt.Errorf("parse position id: %w", err)
	}

	tripID, err := uuid.Parse(p.TripID)
	if err != nil {
		return api.TripPosition{}, fmt.Errorf("parse trip id: %w", err)
	}

	return api.TripPosition{
		Coordinates: api.Coordinates{
			Latitude:  p.CurPoint.Latitude,
			Longitude: p.CurPoint.Longitude,
		},
		Id:         id,
		RecordedAt: p.RecordedAt,
		TripId:     tripID,
	}, nil
}

package get

import (
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
)

func toAPI(positions []*trip.Position) (api.TripPositionsResponse, error) {
	response := api.TripPositionsResponse{
		Positions: make([]api.TripPosition, 0, len(positions)),
	}
	for _, p := range positions {
		id, err := strconv.ParseInt(p.ID, 10, 64)
		if err != nil {
			return api.TripPositionsResponse{}, fmt.Errorf("parse position id: %w", err)
		}

		tripID, err := uuid.Parse(p.TripID)
		if err != nil {
			return api.TripPositionsResponse{}, fmt.Errorf("parse trip id: %w", err)
		}

		response.Positions = append(response.Positions, api.TripPosition{
			Coordinates: api.Coordinates{
				Latitude:  p.CurPoint.Latitude,
				Longitude: p.CurPoint.Longitude,
			},
			Id:         id,
			RecordedAt: p.RecordedAt,
			TripId:     tripID,
		})
	}
	return response, nil
}

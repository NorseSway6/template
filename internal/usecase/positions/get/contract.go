package get

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

type tripRepository interface {
	GetByID(ctx context.Context, tripID string) (*trip.Trip, error)
	ListPositions(ctx context.Context, tripID string) ([]*trip.Position, error)
}

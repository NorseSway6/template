package get

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

type usecase interface {
	Do(ctx context.Context, tripID string) (*trip.Trip, error)
}

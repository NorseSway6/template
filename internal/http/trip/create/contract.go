package create

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/model"
)

type usecase interface {
	Do(ctx context.Context, in model.CreateTripInput) (*trip.Trip, error)
}

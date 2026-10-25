package finish

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

type (
	tripRepository interface {
		GetByID(ctx context.Context, tripID string) (*trip.Trip, error)
		Save(ctx context.Context, t *trip.Trip) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
)

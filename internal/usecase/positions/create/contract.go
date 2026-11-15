package create

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create

type (
	tripRepository interface {
		SavePosition(ctx context.Context, p *trip.Position) error
		GetByID(ctx context.Context, tripID string) (*trip.Trip, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	metrics interface {
		PositionSaved(ctx context.Context)
	}
)

package create

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/model"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create

type (
	usecase interface {
		Do(ctx context.Context, in model.CreatePositionInput) (*trip.Position, error)
	}

	queueCanceller interface {
		Cancel(tripID string)
	}
)

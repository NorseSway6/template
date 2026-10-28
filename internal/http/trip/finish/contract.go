package finish

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish

type usecase interface {
	Do(ctx context.Context, tripID string) (*trip.Trip, error)
}

package get

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=get

type usecase interface {
	Do(ctx context.Context, tripID string) (*trip.Trip, error)
}

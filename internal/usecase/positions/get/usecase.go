package get

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

// Usecase представляет бизнес-логику для получения маршрута поездки.
type Usecase struct {
	repo tripRepository
}

// New создает новый экземпляр Usecase.
func New(repo tripRepository) *Usecase {
	return &Usecase{repo: repo}
}

// Do получает все позиции поездки по указанному tripID.
func (u *Usecase) Do(ctx context.Context, tripID string) ([]*trip.Position, error) {
	if _, err := u.repo.GetByID(ctx, tripID); err != nil {
		return nil, err
	}
	return u.repo.ListPositions(ctx, tripID)
}

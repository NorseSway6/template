package get

import (
	"context"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

// Usecase представляет бизнес-логику для получения информации о поездке.
type Usecase struct {
	repo tripRepository
}

// New создает новый экземпляр Usecase.
func New(repo tripRepository) *Usecase {
	return &Usecase{repo: repo}
}

// Do получает информацию о поездке по указанному tripID.
func (u *Usecase) Do(ctx context.Context, tripID string) (*trip.Trip, error) {
	return u.repo.GetByID(ctx, tripID)
}

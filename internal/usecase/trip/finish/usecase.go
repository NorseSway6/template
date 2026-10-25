package finish

import (
	"context"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

// Usecase представляет бизнес-логику для завершения поездки.
type Usecase struct {
	repo tripRepository
	tx   txManager
	now  func() time.Time
}

// New создает новый экземпляр Usecase.
func New(repo tripRepository, txManager txManager, now func() time.Time) *Usecase {
	return &Usecase{repo: repo, tx: txManager, now: now}
}

// Do завершает поездку указанного tripID.
func (u *Usecase) Do(ctx context.Context, tripID string) (*trip.Trip, error) {
	var finishedTrip *trip.Trip

	err := u.tx.Do(ctx, func(ctx context.Context) error {
		t, err := u.repo.GetByID(ctx, tripID)
		if err != nil {
			return err
		}

		if err := t.Finish(u.now()); err != nil {
			return err
		}

		if err := u.repo.Save(ctx, t); err != nil {
			return err
		}

		finishedTrip = t
		return nil
	})

	return finishedTrip, err
}

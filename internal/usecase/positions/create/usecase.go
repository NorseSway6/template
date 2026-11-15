package create

import (
	"context"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/model"
)

// Usecase представляет бизнес-логику для создания поездки.
type Usecase struct {
	repo    tripRepository
	tx      txManager
	metrics metrics
	now     func() time.Time
}

// New создает новый экземпляр Usecase.
func New(repo tripRepository, txManager txManager, now func() time.Time, metrics metrics) *Usecase {
	return &Usecase{repo: repo, tx: txManager, now: now, metrics: metrics}
}

// Do сохраняет позицию на основе входных данных.
func (u *Usecase) Do(ctx context.Context, in model.CreatePositionInput) (*trip.Position, error) {
	p, err := trip.NewPosition(in.TripID, in.CurrentPoint, in.RecordedAt, u.now())
	if err != nil {
		return nil, err
	}

	err = u.tx.Do(ctx, func(ctx context.Context) error {
		t, err := u.repo.GetByID(ctx, in.TripID)
		if err != nil {
			return err
		}
		if err := t.CheckAcceptsPositions(); err != nil {
			return err
		}

		return u.repo.SavePosition(ctx, p)
	})
	if err != nil {
		return nil, err
	}

	u.metrics.PositionSaved(ctx)
	return p, nil
}

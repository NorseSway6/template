package finish

import (
	"context"
	"log/slog"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
)

// Usecase представляет бизнес-логику для завершения поездки.
type Usecase struct {
	repo    tripRepository
	tx      txManager
	metrics metrics
	logger  *slog.Logger
	now     func() time.Time
}

// New создает новый экземпляр Usecase.
func New(repo tripRepository, txManager txManager, now func() time.Time, metrics metrics, logger *slog.Logger) *Usecase {
	return &Usecase{repo: repo, tx: txManager, now: now, metrics: metrics, logger: logger}
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

	if err == nil {
		u.metrics.TripCompleted()
		u.logger.InfoContext(ctx, "trip completed",
			"event", "trip.completed",
			"trip_id", finishedTrip.ID,
		)
	}

	return finishedTrip, err
}

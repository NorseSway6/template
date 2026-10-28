package create

import (
	"context"
	"log/slog"
	"time"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/model"
)

// Usecase представляет бизнес-логику для создания поездки.
type Usecase struct {
	repo    tripRepository
	tx      txManager
	metrics metrics
	logger  *slog.Logger
	now     func() time.Time
	newID   func() string
}

// New создает новый экземпляр Usecase.
func New(repo tripRepository, txManager txManager, now func() time.Time, newID func() string, metrics metrics, logger *slog.Logger) *Usecase {
	return &Usecase{repo: repo, tx: txManager, now: now, newID: newID, metrics: metrics, logger: logger}
}

// Do создает новую поездку на основе входных данных.
func (u *Usecase) Do(ctx context.Context, in model.CreateTripInput) (*trip.Trip, error) {
	t, err := trip.New(u.newID(), in.UserID, in.DriverID, in.Price, in.StartPoint, in.EndPoint, u.now())
	if err != nil {
		return nil, err
	}

	err = u.tx.Do(ctx, func(ctx context.Context) error {
		return u.repo.Create(ctx, t)
	})
	if err != nil {
		return nil, err
	}

	u.metrics.TripCreated()
	u.logger.InfoContext(ctx, "trip created",
		"event", "trip.created",
		"trip_id", t.ID,
	)

	return t, nil
}

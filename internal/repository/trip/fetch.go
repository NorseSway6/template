package trip

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
)

// GetByID получает поездку по tripID.
func (r *Repository) GetByID(ctx context.Context, tripID string) (*trip.Trip, error) {
	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude",
		"end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at",
		"created_at", "updated_at",
	).
		From("trips").
		Where(squirrel.Eq{"id": tripID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select trip sql: %w", err)
	}

	var row tripRow
	err = exec.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.UserID, &row.DriverID,
		&row.StartLatitude, &row.StartLongitude,
		&row.EndLatitude, &row.EndLongitude,
		&row.Price, &row.Status, &row.StartedAt, &row.FinishedAt,
		&row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, trip.ErrTripNotFound
		}
		return nil, fmt.Errorf("scan trip: %w", err)
	}

	return fromDomain(&row), nil
}

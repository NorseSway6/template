package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	"github.com/NorseSway6/template.git/internal/model"
)

// GetByID получает поездку по tripID.
func (r *Repository) GetByID(ctx context.Context, tripID string) (*trip.Trip, error) {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude",
		"end_latitude", "end_longitude",
		"price", statusColumn, "started_at", "finished_at",
		"created_at", "updated_at", "last_position_at",
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
		&row.CreatedAt, &row.UpdatedAt, &row.LastPosAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, trip.ErrTripNotFound
		}
		return nil, fmt.Errorf("scan trip: %w", err)
	}

	return toDomain(&row), nil
}

// ListPositions получает позиции поездки в порядке фиксации на устройстве.
func (r *Repository) ListPositions(ctx context.Context, tripID string) ([]*trip.Position, error) {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Select(
		"id", "trip_id", "latitude",
		"longitude", "recorded_at", "created_at",
	).
		From("trip_positions").
		Where(squirrel.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select positions sql: %w", err)
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query trip positions: %w", err)
	}
	defer rows.Close()

	positions := make([]*trip.Position, 0)
	for rows.Next() {
		var row positionRow
		if err := rows.Scan(
			&row.ID, &row.TripID, &row.Latitude,
			&row.Longitude, &row.RecordedAt, &row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan trip position: %w", err)
		}
		positions = append(positions, toDomainPosition(&row))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trip positions: %w", err)
	}

	return positions, nil
}

// ListStaleBatch возвращает следующую страницу поездок, по которым давно не было координат.
func (r *Repository) ListStaleBatch(
	ctx context.Context,
	staleAfter time.Duration,
	after *model.StaleTrip,
	limit int,
) ([]model.StaleTrip, error) {
	if staleAfter <= 0 {
		return nil, fmt.Errorf("staleAfter must be positive, got %s", staleAfter)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("stale trip batch limit must be positive, got %d", limit)
	}

	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)

	builder := r.sq.Select(
		"id", "driver_id", "COALESCE(last_position_at, started_at) AS stale_since",
	).
		From("trips").
		Where(squirrel.And{
			squirrel.Expr(
				"COALESCE(last_position_at, started_at) < NOW() - (? * INTERVAL '1 microsecond')",
				staleAfter.Microseconds(),
			),
			squirrel.Eq{statusColumn: trip.StatusActive},
		})
	if after != nil {
		builder = builder.Where(squirrel.Or{
			squirrel.Expr("COALESCE(last_position_at, started_at) > ?", after.StaleSince),
			squirrel.And{
				squirrel.Expr("COALESCE(last_position_at, started_at) = ?", after.StaleSince),
				squirrel.Expr("id > ?", after.ID),
			},
		})
	}
	query, args, err := builder.
		OrderBy("stale_since ASC", "id ASC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select stale trips sql: %w", err)
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query stale trips: %w", err)
	}
	defer rows.Close()

	var trips []model.StaleTrip
	for rows.Next() {
		var trip model.StaleTrip
		if err := rows.Scan(&trip.ID, &trip.DriverID, &trip.StaleSince); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}
		trips = append(trips, trip)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale trips rows: %w", err)
	}

	return trips, nil
}

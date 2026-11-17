package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
)

// Create создает новую поездку и пишет в журнал переходов статуса.
func (r *Repository) Create(ctx context.Context, t *trip.Trip) error {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)
	row := fromDomain(t)

	query, args, err := r.sq.Insert("trips").
		Columns("id", "user_id", "driver_id", "start_latitude", "start_longitude",
			"end_latitude", "end_longitude", "price", statusColumn, "started_at").
		Values(row.ID, row.UserID, row.DriverID, row.StartLatitude, row.StartLongitude,
			row.EndLatitude, row.EndLongitude, row.Price, row.Status, row.StartedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip sql: %w", err)
	}

	if _, err := exec.Exec(ctx, query, args...); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return trip.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}

	return r.insertHistory(ctx, exec, t.ID, nil, t.Status, "trip created")
}

// Save обновляет статус поездки и пишет в журнал переходов статуса.
func (r *Repository) Save(ctx context.Context, t *trip.Trip) error {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Update("trips").
		Set(statusColumn, t.Status).
		Set("finished_at", t.FinishedAt).
		Set("updated_at", t.UpdatedAt).
		Where(squirrel.Eq{"id": t.ID, statusColumn: trip.StatusActive}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update trip sql: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("execute update trip: %w", err)
	}

	if tag.RowsAffected() == 0 {
		selectQuery, selectArgs, err := r.sq.Select(statusColumn).
			From("trips").
			Where(squirrel.Eq{"id": t.ID}).
			ToSql()
		if err != nil {
			return fmt.Errorf("build select trip status sql: %w", err)
		}

		var status string
		err = exec.QueryRow(ctx, selectQuery, selectArgs...).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return trip.ErrTripNotFound
		}
		if err != nil {
			return fmt.Errorf("select trip status after failed update: %w", err)
		}
		if status == trip.StatusCompleted {
			return trip.ErrTripCompleted
		}
		return fmt.Errorf("trip %s: no active row to finish", t.ID)
	}

	from := trip.StatusActive
	return r.insertHistory(ctx, exec, t.ID, &from, t.Status, "trip finished")
}

// SavePosition сохраняет позицию поездки в базе данных.
func (r *Repository) SavePosition(ctx context.Context, p *trip.Position) error {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)
	row := fromDomainPosition(p)

	query, args, err := r.sq.Insert("trip_positions").
		Columns("trip_id", "latitude", "longitude", "recorded_at").
		Values(row.TripID, row.Latitude, row.Longitude, row.RecordedAt).
		Suffix("RETURNING id, created_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert position sql: %w", err)
	}

	if err := exec.QueryRow(ctx, query, args...).Scan(&p.ID, &p.CreatedAt); err != nil {
		return fmt.Errorf("insert position: %w", err)
	}

	query, args, err = r.sq.Update("trips").
		Set("last_position_at", squirrel.Expr("GREATEST(COALESCE(last_position_at, ?), ?)", row.RecordedAt, row.RecordedAt)).
		Where(squirrel.Eq{"id": row.TripID, statusColumn: trip.StatusActive}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update last_position_at sql: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update last_position_at: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return trip.ErrTripCompleted
	}

	return nil
}

// UpdateLastPositionAt обновляет поле last_position_at в таблице trips.
func (r *Repository) UpdateLastPositionAt(ctx context.Context, tripID string, lastPosAt time.Time) error {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Update("trips").
		Set("last_position_at", lastPosAt).
		Where(squirrel.Eq{"id": tripID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update last_position_at sql: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("execute update last_position_at: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return trip.ErrTripNotFound
	}

	return nil
}

// insertHistory добавляет запись в журнал переходов статуса поездки.
func (r *Repository) insertHistory(
	ctx context.Context,
	exec dbtx,
	tripID string,
	from *string,
	to, reason string,
) error {
	query, args, err := r.sq.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(tripID, from, to, reason).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert history sql: %w", err)
	}

	if _, err := exec.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("execute insert history: %w", err)
	}

	return nil
}

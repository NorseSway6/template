package trip

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
)

// Create создает новую поездку и пишет в журнал переходов статуса.
func (r *Repository) Create(ctx context.Context, t *trip.Trip) error {
	exec := transaction.GetExecutor(ctx, r.pool)
	row := toDomain(t)

	query, args, err := r.sq.Insert("trips").
		Columns("id", "user_id", "driver_id", "start_latitude", "start_longitude",
			"end_latitude", "end_longitude", "price", "status", "started_at").
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
	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Update("trips").
		Set("status", t.Status).
		Set("finished_at", t.FinishedAt).
		Set("updated_at", t.UpdatedAt).
		Where(squirrel.Eq{"id": t.ID, "status": trip.StatusActive}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update trip sql: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("execute update trip: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("trip %s: no active row to finish", t.ID)
	}

	from := trip.StatusActive
	return r.insertHistory(ctx, exec, t.ID, &from, t.Status, "trip finished")
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

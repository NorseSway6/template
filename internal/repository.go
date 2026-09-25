package internal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
    StatusActive    = "active"
    StatusCompleted = "completed"
)

// TripRepository реализует работу с таблицами trips и trip_status_history.
type TripRepository struct {
	pool *pgxpool.Pool
	sq squirrel.StatementBuilderType
}

// NewTripRepository создает новый экземпляр репозитория.
func NewTripRepository(db *pgxpool.Pool) *TripRepository {
	return &TripRepository{
		pool: db,
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// Trip представляет модель данных для талицы trips.
type Trip struct {
	ID             string
	UserID         string
	DriverID       string
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Create создает новую поездку и пишет в журнал переходов статуса.
func (tr *TripRepository) Create(ctx context.Context, trip *Trip) error {
	exec := GetExecutor(ctx, tr.pool)

	// создание новой поездки
	query, args, err := tr.sq.Insert("trips").
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude",
			"end_latitude", "end_longitude",
			"price", "status", "started_at",
		).
		Values(
			trip.ID, trip.UserID, trip.DriverID,
			trip.StartLatitude, trip.StartLongitude,
			trip.EndLatitude, trip.EndLongitude,
			trip.Price, trip.Status, trip.StartedAt,
		).
		ToSql()

	if err != nil {
		return fmt.Errorf("build insert trip sql: %w", err)
	}

	// вставка поездки
	_, err = exec.Exec(ctx, query, args...)
	if err != nil {	
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDriverBusy
		}
		return fmt.Errorf("execute insert trip: %w", err)
	}

	// запись о старте в журнал
	historyQuery, historyArgs, err := tr.sq.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(trip.ID, nil, trip.Status, "trip created").
		ToSql()

	if err != nil {
		return fmt.Errorf("build insert history sql: %w", err)
	}

	_, err = exec.Exec(ctx, historyQuery, historyArgs...)
	if err != nil {
		return fmt.Errorf("execute insert history: %w", err)
	}

	return nil
}

// GetByID получает поездку по ее UUID.
func (tr *TripRepository) GetByID(ctx context.Context, tripID string) (*Trip, error) {
	exec := GetExecutor(ctx, tr.pool)

	query, args, err := tr.sq.Select(
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

	var t Trip
	row := exec.QueryRow(ctx, query, args...)
	
	err = row.Scan(
		&t.ID, &t.UserID, &t.DriverID,
		&t.StartLatitude, &t.StartLongitude,
		&t.EndLatitude, &t.EndLongitude,
		&t.Price, &t.Status, &t.StartedAt, &t.FinishedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTripNotFound
		}
		return nil, fmt.Errorf("scan trip: %w", err)
	}

	return &t, nil
}

// Finish завершает поездку путем перевода статуса в completed и проставления finished_at.
func (tr *TripRepository) Finish(ctx context.Context, tripID string, finishedAt time.Time) (*Trip, error) {
	exec := GetExecutor(ctx, tr.pool)

	// обновление статуса поездки
	query, args, err := tr.sq.Update("trips").
		Set("status", StatusCompleted).
		Set("finished_at", finishedAt).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": tripID, "status": StatusActive}).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("build update trip sql: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execute update trip: %w", err)
	}

	if tag.RowsAffected() == 0 {
		existing, getErr := tr.GetByID(ctx, tripID)
		if getErr != nil {
			return nil, getErr
		}
		if existing.Status == StatusCompleted {
			return nil, ErrTripCompleted
		}
		return nil, ErrTripNotFound
	}

	// запись истории изменения статуса
	historyQuery, historyArgs, err := tr.sq.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(tripID, StatusActive, StatusCompleted, "trip finished").
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("build finish history sql: %w", err)
	}

	_, err = exec.Exec(ctx, historyQuery, historyArgs...)
	if err != nil {
		return nil, fmt.Errorf("execute finish history: %w", err)
	}

	return tr.GetByID(ctx, tripID)
}


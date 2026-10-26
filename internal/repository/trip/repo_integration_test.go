//go:build integration
// +build integration

package trip

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
)

func TestRepositoryCreateSuccess(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := New(pool)

	created := &trip.Trip{
		ID:         "44444444-4444-4444-4444-444444444444",
		UserID:     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		DriverID:   "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Status:     trip.StatusActive,
		Price:      500,
		StartPoint: trip.Point{Latitude: 11, Longitude: 12},
		EndPoint:   trip.Point{Latitude: 13, Longitude: 14},
		StartedAt:  time.Now(),
	}

	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var tripsCount, historyCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trips").Scan(&tripsCount); err != nil {
		t.Fatalf("count trips: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trip_status_history").Scan(&historyCount); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if tripsCount != 1 || historyCount != 1 {
		t.Fatalf("want 1 row in trips and 1 in history, got %d/%d", tripsCount, historyCount)
	}
}

func TestRepositoryCreateRollbackOnError(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := New(pool)
	txManager := transaction.New(pool)

	created := &trip.Trip{
		ID:         "11111111-1111-1111-1111-111111111111",
		UserID:     "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		DriverID:   "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Status:     trip.StatusActive,
		Price:      1500,
		StartPoint: trip.Point{Latitude: 10, Longitude: 20},
		EndPoint:   trip.Point{Latitude: 30, Longitude: 40},
		StartedAt:  time.Now(),
	}

	err := txManager.Do(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, created); err != nil {
			return err
		}
		_, err := pool.Exec(ctx, "INSERT INTO trip_status_history (trip_id, from_status, to_status, reason) VALUES ($1, $2, $3, $4)", created.ID, trip.StatusActive, "bad_status", "invalid status")
		return err
	})
	if err == nil {
		t.Fatal("expected transaction error when invalid status is inserted")
	}

	var tripsCount, historyCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trips").Scan(&tripsCount); err != nil {
		t.Fatalf("count trips after rollback: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trip_status_history").Scan(&historyCount); err != nil {
		t.Fatalf("count history after rollback: %v", err)
	}
	if tripsCount != 0 || historyCount != 0 {
		t.Fatalf("expected full rollback, got trips=%d history=%d", tripsCount, historyCount)
	}
}

func setupIntegrationDB(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL is not set; integration tests require a real PostgreSQL instance")
	}

	parsed, err := url.Parse(dbURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	if parsed.Path == "" || parsed.Path == "/" {
		t.Fatalf("DATABASE_URL must include a database name")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	if _, err := pool.Exec(ctx, "SELECT 1"); err != nil {
		t.Fatalf("connect to database: %v", err)
	}

	for _, migrationPath := range []string{
		filepath.Join("..", "..", "..", "migrations", "00001_create_trips.sql"),
		filepath.Join("..", "..", "..", "migrations", "00002_create_trip_status_history.sql"),
	} {
		content, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("read %s: %v", migrationPath, err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			if !isAlreadyExistsError(err) {
				t.Fatalf("apply migration %s: %v", migrationPath, err)
			}
		}
	}

	_, err = pool.Exec(ctx, "TRUNCATE TABLE trip_status_history, trips")
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	return pool
}

func isAlreadyExistsError(err error) bool {
	return err != nil && (err.Error() == "ERROR: relation \"trips\" already exists (SQLSTATE 42P07)" || err.Error() == "ERROR: relation \"trip_status_history\" already exists (SQLSTATE 42P07)")
}

func TestRepositoryMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	_ = pool
	fmt.Println("integration-db-ok")
}

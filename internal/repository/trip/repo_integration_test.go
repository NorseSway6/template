//go:build integration
// +build integration

package trip

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	httpidem "github.com/NorseSway6/template.git/internal/http/idempotency"
	createhandler "github.com/NorseSway6/template.git/internal/http/trip/create"
	finishhandler "github.com/NorseSway6/template.git/internal/http/trip/finish"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	"github.com/NorseSway6/template.git/internal/model"
	idemrepo "github.com/NorseSway6/template.git/internal/repository/idempotency"
	createuc "github.com/NorseSway6/template.git/internal/usecase/trip/create"
	finishuc "github.com/NorseSway6/template.git/internal/usecase/trip/finish"
)

const (
	integrationUserID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	integrationDriverID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func TestRepositoryCreateSuccess(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := New(pool, 3*time.Second)

	created := &domaintrip.Trip{
		ID:         "44444444-4444-4444-4444-444444444444",
		UserID:     integrationUserID,
		DriverID:   integrationDriverID,
		Status:     domaintrip.StatusActive,
		Price:      500,
		StartPoint: domaintrip.Point{Latitude: 11, Longitude: 12},
		EndPoint:   domaintrip.Point{Latitude: 13, Longitude: 14},
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

func TestRepositoryGetByID(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := New(pool, 3*time.Second)

	created := &domaintrip.Trip{
		ID:         "77777777-7777-7777-7777-777777777777",
		UserID:     integrationUserID,
		DriverID:   integrationDriverID,
		Status:     domaintrip.StatusActive,
		Price:      1200,
		StartPoint: domaintrip.Point{Latitude: 11, Longitude: 12},
		EndPoint:   domaintrip.Point{Latitude: 13, Longitude: 14},
		StartedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := repo.Create(ctx, created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != created.ID || got.Status != domaintrip.StatusActive ||
		got.UserID != created.UserID || got.DriverID != created.DriverID {
		t.Fatalf("GetByID() = %#v, want trip %q with active status", got, created.ID)
	}
}

func TestRepositoryCreateReturnsDriverBusy(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := New(pool, 3*time.Second)
	now := time.Now()
	first := &domaintrip.Trip{
		ID:         "88888888-8888-8888-8888-888888888888",
		UserID:     integrationUserID,
		DriverID:   integrationDriverID,
		Status:     domaintrip.StatusActive,
		Price:      500,
		StartPoint: domaintrip.Point{Latitude: 11, Longitude: 12},
		EndPoint:   domaintrip.Point{Latitude: 13, Longitude: 14},
		StartedAt:  now,
	}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create() first trip error = %v", err)
	}

	second := *first
	second.ID = "99999999-9999-9999-9999-999999999999"
	err := repo.Create(ctx, &second)
	if !errors.Is(err, domaintrip.ErrDriverBusy) {
		t.Fatalf("Create() second trip error = %v, want %v", err, domaintrip.ErrDriverBusy)
	}
}

func TestRepositoryCreateRollbackOnError(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := New(pool, 3*time.Second)
	txManager := transaction.New(pool, 3*time.Second)

	created := &domaintrip.Trip{
		ID:         "11111111-1111-1111-1111-111111111111",
		UserID:     integrationUserID,
		DriverID:   integrationDriverID,
		Status:     domaintrip.StatusActive,
		Price:      1500,
		StartPoint: domaintrip.Point{Latitude: 10, Longitude: 20},
		EndPoint:   domaintrip.Point{Latitude: 30, Longitude: 40},
		StartedAt:  time.Now(),
	}

	err := txManager.Do(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, created); err != nil {
			return err
		}
		_, err := pool.Exec(ctx, "INSERT INTO trip_status_history (trip_id, from_status, to_status, reason) VALUES ($1, $2, $3, $4)", created.ID, domaintrip.StatusActive, "bad_status", "invalid status")
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

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("parse test pool config: %v", err)
	}

	adminPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("create admin pool: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "SELECT 1"); err != nil {
		adminPool.Close()
		t.Fatalf("connect to database: %v", err)
	}

	schemaName := "trip_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	schema := pgx.Identifier{schemaName}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		adminPool.Close()
		t.Fatalf("create test schema: %v", err)
	}

	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		_, _ = adminPool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		adminPool.Close()
		t.Fatalf("create isolated test pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := adminPool.Exec(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		adminPool.Close()
	})

	for _, migrationPath := range []string{
		filepath.Join("..", "..", "..", "migrations", "00001_create_trips.sql"),
		filepath.Join("..", "..", "..", "migrations", "00002_create_trip_status_history.sql"),
		filepath.Join("..", "..", "..", "migrations", "00003_create_idempotency_keys.sql"),
	} {
		content, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("read %s: %v", migrationPath, err)
		}
		upSQL, _, hasDownSection := strings.Cut(string(content), "-- +goose Down")
		if !hasDownSection {
			t.Fatalf("migration %s has no goose Down marker", migrationPath)
		}
		if _, err := pool.Exec(ctx, upSQL); err != nil {
			t.Fatalf("apply migration %s: %v", migrationPath, err)
		}
	}

	return pool
}

func TestRepositorySaveReturnsTripNotFound(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := New(pool, 3*time.Second)

	err := repo.Save(ctx, &domaintrip.Trip{
		ID:     "55555555-5555-5555-5555-555555555555",
		Status: domaintrip.StatusCompleted,
	})
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("Save() error = %v, want %v", err, domaintrip.ErrTripNotFound)
	}
}

func TestConcurrentIdempotentCreateReturnsSameTrip(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	txManager := transaction.New(pool, 3*time.Second)
	tripRepo := New(pool, 3*time.Second)
	usecase := createuc.New(
		tripRepo,
		txManager,
		time.Now,
		uuid.NewString,
		idempotencyTestMetrics{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	create := createhandler.New(usecase)
	idempotency := httpidem.New(idemrepo.New(pool, 3*time.Second, 24*time.Hour), txManager)
	handler := idempotency.Wrap(func(w http.ResponseWriter, r *http.Request) {
		create.Create(w, r, api.CreateTripParams{})
	})

	key := uuid.NewString()
	body := `{"user_id":"11111111-1111-1111-1111-111111111111","driver_id":"22222222-2222-2222-2222-222222222222","start_point":{"latitude":10,"longitude":20},"end_point":{"latitude":30,"longitude":40},"price":1500}`
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		go func() {
			<-start
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body))
			req.Header.Set("Idempotency-Key", key)
			res := httptest.NewRecorder()
			handler(res, req)
			responses <- res
		}()
	}
	close(start)

	first, second := <-responses, <-responses
	statuses := map[int]int{first.Code: 1}
	statuses[second.Code]++
	if statuses[http.StatusCreated] != 1 || statuses[http.StatusOK] != 1 {
		t.Fatalf("response statuses = %v, want one 201 and one 200", statuses)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("response bodies differ:\nfirst:  %s\nsecond: %s", first.Body.String(), second.Body.String())
	}

	var tripsCount, historyCount, idempotencyCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trips").Scan(&tripsCount); err != nil {
		t.Fatalf("count trips: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trip_status_history").Scan(&historyCount); err != nil {
		t.Fatalf("count trip history: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM idempotency_keys WHERE key = $1", key).Scan(&idempotencyCount); err != nil {
		t.Fatalf("count idempotency keys: %v", err)
	}
	if tripsCount != 1 || historyCount != 1 || idempotencyCount != 1 {
		t.Fatalf("created rows = trips:%d history:%d keys:%d, want 1 each", tripsCount, historyCount, idempotencyCount)
	}
}

func TestExpiredIdempotencyKeyCanBeReused(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := idemrepo.New(pool, 3*time.Second, 24*time.Hour)
	txManager := transaction.New(pool, 3*time.Second)
	key := uuid.NewString()
	err := txManager.Do(ctx, func(ctx context.Context) error {
		claimed, _, err := repo.Claim(ctx, key, "first")
		if err != nil {
			return err
		}
		if !claimed {
			t.Fatal("first Claim() was not claimed")
		}
		return repo.Complete(ctx, &model.IdempotencyRecord{
			Key: key, RequestHash: "first", ResponseCode: http.StatusOK, ResponseBody: []byte("first response"),
		})
	})
	if err != nil {
		t.Fatalf("save first idempotency key: %v", err)
	}

	if _, err := pool.Exec(ctx, "UPDATE idempotency_keys SET created_at = NOW() - INTERVAL '25 hours' WHERE key = $1", key); err != nil {
		t.Fatalf("expire idempotency key: %v", err)
	}

	err = txManager.Do(ctx, func(ctx context.Context) error {
		claimed, _, err := repo.Claim(ctx, key, "second")
		if err != nil {
			return err
		}
		if !claimed {
			t.Fatal("Claim() did not replace expired key")
		}
		return repo.Complete(ctx, &model.IdempotencyRecord{
			Key: key, RequestHash: "second", ResponseCode: http.StatusOK, ResponseBody: []byte("second response"),
		})
	})
	if err != nil {
		t.Fatalf("reuse expired idempotency key: %v", err)
	}

	err = txManager.Do(ctx, func(ctx context.Context) error {
		claimed, record, err := repo.Claim(ctx, key, "second")
		if err != nil {
			return err
		}
		if claimed || record == nil || record.RequestHash != "second" ||
			string(record.ResponseBody) != "second response" {
			t.Fatalf("record after key reuse = (%t, %#v), want existing second request", claimed, record)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read idempotency key after reuse: %v", err)
	}
}

func TestExpiredIdempotencyRecordsAreDeleted(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	repo := idemrepo.New(pool, 3*time.Second, 24*time.Hour)
	txManager := transaction.New(pool, 3*time.Second)
	expiredKey := uuid.NewString()
	activeKey := uuid.NewString()

	for _, key := range []string{expiredKey, activeKey} {
		err := txManager.Do(ctx, func(ctx context.Context) error {
			claimed, _, err := repo.Claim(ctx, key, key)
			if err != nil {
				return err
			}
			if !claimed {
				t.Fatalf("Claim(%q) was not claimed", key)
			}
			return repo.Complete(ctx, &model.IdempotencyRecord{
				Key: key, RequestHash: key, ResponseCode: http.StatusOK, ResponseBody: []byte("response"),
			})
		})
		if err != nil {
			t.Fatalf("save idempotency key %q: %v", key, err)
		}
	}

	if _, err := pool.Exec(ctx,
		"UPDATE idempotency_keys SET created_at = NOW() - INTERVAL '25 hours' WHERE key = $1",
		expiredKey,
	); err != nil {
		t.Fatalf("expire idempotency key: %v", err)
	}

	err := txManager.Do(ctx, func(ctx context.Context) error {
		claimed, record, err := repo.Claim(ctx, activeKey, activeKey)
		if err != nil {
			return err
		}
		if claimed || record == nil || record.Key != activeKey {
			t.Fatalf("Claim(active key) = (%t, %#v), want existing active key", claimed, record)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("claim active idempotency key: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM idempotency_keys WHERE key = $1", expiredKey).Scan(&count); err != nil {
		t.Fatalf("count expired idempotency key: %v", err)
	}
	if count != 0 {
		t.Fatalf("expired key rows = %d, want 0", count)
	}
}

func TestConcurrentFinishReturnsSuccessAndConflict(t *testing.T) {
	ctx := context.Background()
	pool := setupIntegrationDB(t, ctx)
	baseRepo := New(pool, 3*time.Second)
	txManager := transaction.New(pool, 3*time.Second)
	tripID := "66666666-6666-6666-6666-666666666666"

	created := &domaintrip.Trip{
		ID:         tripID,
		UserID:     integrationUserID,
		DriverID:   integrationDriverID,
		Status:     domaintrip.StatusActive,
		Price:      500,
		StartPoint: domaintrip.Point{Latitude: 11, Longitude: 12},
		EndPoint:   domaintrip.Point{Latitude: 13, Longitude: 14},
		StartedAt:  time.Now(),
	}
	if err := baseRepo.Create(ctx, created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	repo := &synchronizedFinishRepository{
		base:    baseRepo,
		arrived: make(chan struct{}, 2),
		resume:  make(chan struct{}),
	}
	metrics := &finishMetrics{}
	uc := finishuc.New(repo, txManager, time.Now, metrics, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := finishhandler.New(uc)
	responses := make(chan *httptest.ResponseRecorder, 2)

	for range 2 {
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID+"/finish", nil)
			res := httptest.NewRecorder()
			handler.Finish(res, req, uuid.MustParse(tripID))
			responses <- res
		}()
	}

	for range 2 {
		select {
		case <-repo.arrived:
		case <-time.After(10 * time.Second):
			close(repo.resume)
			t.Fatal("timed out waiting for both requests to read the active trip")
		}
	}
	close(repo.resume)

	statuses := map[int]int{}
	for range 2 {
		select {
		case res := <-responses:
			statuses[res.Code]++
			if res.Code == http.StatusConflict && !strings.Contains(res.Body.String(), `"code":"trip_completed"`) {
				t.Fatalf("conflict response body = %s, want trip_completed code", res.Body.String())
			}
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for finish response")
		}
	}
	if statuses[http.StatusOK] != 1 || statuses[http.StatusConflict] != 1 {
		t.Fatalf("response statuses = %v, want one 200 and one 409", statuses)
	}

	finished, err := baseRepo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("GetByID() after finish error = %v", err)
	}
	if finished.Status != domaintrip.StatusCompleted || finished.FinishedAt == nil {
		t.Fatalf("finished trip = %#v, want completed status and finished_at", finished)
	}
	var historyCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trip_status_history WHERE trip_id = $1", tripID).Scan(&historyCount); err != nil {
		t.Fatalf("count finish history: %v", err)
	}
	if historyCount != 2 {
		t.Fatalf("history count = %d, want creation and finish entries", historyCount)
	}
}

type synchronizedFinishRepository struct {
	base    *Repository
	arrived chan struct{}
	resume  chan struct{}
}

func (r *synchronizedFinishRepository) GetByID(ctx context.Context, tripID string) (*domaintrip.Trip, error) {
	t, err := r.base.GetByID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	r.arrived <- struct{}{}
	select {
	case <-r.resume:
		return t, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *synchronizedFinishRepository) Save(ctx context.Context, t *domaintrip.Trip) error {
	return r.base.Save(ctx, t)
}

type finishMetrics struct{}

type idempotencyTestMetrics struct{}

func (idempotencyTestMetrics) TripCreated() {}

func (*finishMetrics) TripCompleted() {}

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

const idempotencyTTL = 24 * time.Hour

// IdempotencyRecord хранит данные об обработанном HTTP-запросе.
type IdempotencyRecord struct {
	Key          string
	RequestHash  string
	ResponseCode int
	ResponseBody []byte
}

// IdempotencyRepository отвечает за сохранение и получение ключей идемпотентности из базы.
type IdempotencyRepository struct {
	pool         *pgxpool.Pool
	sq           squirrel.StatementBuilderType
	queryTimeout time.Duration
}

// NewIdempotencyRepository создает новый экземпляр репозитория.
func NewIdempotencyRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *IdempotencyRepository {
	return &IdempotencyRepository{
		pool:         pool,
		sq:           squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		queryTimeout: queryTimeout,
	}
}

// Get ищет запись идемпотентности по переданному ключу.
func (r *IdempotencyRepository) Get(ctx context.Context, key string) (*IdempotencyRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Select("key", "request_hash", "response_code", "response_body").
		From("idempotency_keys").
		Where(squirrel.Eq{"key": key}).
		Where(squirrel.Gt{"created_at": time.Now().Add(-idempotencyTTL)}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select idempotency sql: %w", err)
	}

	var rec IdempotencyRecord
	err = exec.QueryRow(ctx, query, args...).Scan(&rec.Key, &rec.RequestHash, &rec.ResponseCode, &rec.ResponseBody)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get idempotency record: %w", err)
	}

	return &rec, nil
}

// Save сохраняет результат выполнения запроса в базу для будущих проверок.
func (r *IdempotencyRepository) Save(ctx context.Context, rec *IdempotencyRecord) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := GetExecutor(ctx, r.pool)

	deleteQuery, deleteArgs, err := r.sq.Delete("idempotency_keys").
		Where(squirrel.LtOrEq{"created_at": time.Now().Add(-idempotencyTTL)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete expired idempotency sql: %w", err)
	}
	if _, err := exec.Exec(ctx, deleteQuery, deleteArgs...); err != nil {
		return fmt.Errorf("delete expired idempotency records: %w", err)
	}

	query, args, err := r.sq.Insert("idempotency_keys").
		Columns("key", "request_hash", "response_code", "response_body").
		Values(rec.Key, rec.RequestHash, rec.ResponseCode, rec.ResponseBody).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert idempotency sql: %w", err)
	}

	_, err = exec.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idempotency_keys_pkey" {
			return ErrIdempotencyKeyExists
		}
		return fmt.Errorf("save idempotency record: %w", err)
	}

	return nil
}

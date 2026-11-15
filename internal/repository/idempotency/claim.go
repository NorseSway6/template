package idempotency

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	"github.com/NorseSway6/template.git/internal/model"
)

// Claim захватывает новый или истекший ключ либо возвращает действующую запись.
func (r *Repository) Claim(
	ctx context.Context,
	key, requestHash string,
) (bool, *model.IdempotencyRecord, error) {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Delete("idempotency_keys").
		Where("created_at <= NOW() - (? * INTERVAL '1 second')", r.ttl.Seconds()).
		ToSql()
	if err != nil {
		return false, nil, fmt.Errorf("build delete expired idempotency sql: %w", err)
	}
	if _, err := exec.Exec(ctx, query, args...); err != nil {
		return false, nil, fmt.Errorf("delete expired idempotency records: %w", err)
	}

	query, args, err = r.sq.Insert("idempotency_keys").
		Columns("key", "request_hash", "response_code", "response_body").
		Values(key, requestHash, 0, []byte{}).
		Suffix(`ON CONFLICT ("key") DO UPDATE SET
			request_hash = EXCLUDED.request_hash,
			response_code = 0,
			response_body = EXCLUDED.response_body,
			created_at = NOW()
			WHERE idempotency_keys.created_at <= NOW() - (? * INTERVAL '1 second')
			RETURNING "key", request_hash, response_code, response_body`,
			r.ttl.Seconds(),
		).
		ToSql()
	if err != nil {
		return false, nil, fmt.Errorf("build claim idempotency sql: %w", err)
	}

	var rec model.IdempotencyRecord
	err = exec.QueryRow(ctx, query, args...).Scan(
		&rec.Key, &rec.RequestHash, &rec.ResponseCode, &rec.ResponseBody,
	)
	if err == nil {
		return true, nil, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, nil, fmt.Errorf("claim idempotency record: %w", err)
	}

	query, args, err = r.sq.Select("key", "request_hash", "response_code", "response_body").
		From("idempotency_keys").
		Where(squirrel.Eq{idempotencyKeyColumn: key}).
		Where("created_at > NOW() - (? * INTERVAL '1 second')", r.ttl.Seconds()).
		ToSql()
	if err != nil {
		return false, nil, fmt.Errorf("build select idempotency sql: %w", err)
	}
	err = exec.QueryRow(ctx, query, args...).Scan(
		&rec.Key, &rec.RequestHash, &rec.ResponseCode, &rec.ResponseBody,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, fmt.Errorf("idempotency record disappeared after claim conflict")
		}
		return false, nil, fmt.Errorf("get idempotency record: %w", err)
	}
	return false, &rec, nil
}

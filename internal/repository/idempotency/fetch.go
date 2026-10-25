package idempotency

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	"github.com/NorseSway6/template.git/internal/model"
	"github.com/jackc/pgx/v5"
)

// Get извлекает запись идемпотентности из базы данных по ключу.
func (r *Repository) Get(ctx context.Context, key string) (*model.IdempotencyRecord, error) {
	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Select("key", "request_hash", "response_code", "response_body").
		From("idempotency_keys").
		Where(squirrel.Eq{"key": key}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select idempotency sql: %w", err)
	}

	var rec model.IdempotencyRecord
	err = exec.QueryRow(ctx, query, args...).
		Scan(&rec.Key, &rec.RequestHash, &rec.ResponseCode, &rec.ResponseBody)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get idempotency record: %w", err)
	}

	return &rec, nil
}

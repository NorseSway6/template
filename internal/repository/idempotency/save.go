package idempotency

import (
	"context"
	"fmt"

	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	"github.com/NorseSway6/template.git/internal/model"
)

// Save сохраняет запись идемпотентности в базу данных.
func (r *Repository) Save(ctx context.Context, rec *model.IdempotencyRecord) error {
	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Insert("idempotency_keys").
		Columns("key", "request_hash", "response_code", "response_body").
		Values(rec.Key, rec.RequestHash, rec.ResponseCode, rec.ResponseBody).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert idempotency sql: %w", err)
	}

	if _, err := exec.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("save idempotency record: %w", err)
	}

	return nil
}

package idempotency

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"

	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	"github.com/NorseSway6/template.git/internal/model"
)

// Complete сохраняет успешный ответ для захваченного ключа.
func (r *Repository) Complete(ctx context.Context, rec *model.IdempotencyRecord) error {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Update("idempotency_keys").
		Set("response_code", rec.ResponseCode).
		Set("response_body", rec.ResponseBody).
		Where(squirrel.Eq{
			idempotencyKeyColumn: rec.Key,
			"request_hash":       rec.RequestHash,
			"response_code":      0,
		}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build complete idempotency sql: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("complete idempotency record: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("complete idempotency record: expected one pending key, updated %d", tag.RowsAffected())
	}
	return nil
}

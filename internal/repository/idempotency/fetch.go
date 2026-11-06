package idempotency

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"

	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
)

// Release удаляет захваченный ключ, если обработка запроса не создала ресурс.
func (r *Repository) Release(ctx context.Context, key, requestHash string) error {
	ctx, cancel := transaction.WithQueryTimeout(ctx, r.queryTimeout)
	defer cancel()

	exec := transaction.GetExecutor(ctx, r.pool)

	query, args, err := r.sq.Delete("idempotency_keys").
		Where(squirrel.Eq{idempotencyKeyColumn: key, "request_hash": requestHash, "response_code": 0}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build release idempotency sql: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("release idempotency record: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("release idempotency record: expected one pending key, deleted %d", tag.RowsAffected())
	}
	return nil
}

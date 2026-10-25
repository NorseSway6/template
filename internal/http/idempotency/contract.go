package idempotency

import (
	"context"

	"github.com/NorseSway6/template.git/internal/model"
)

type store interface {
	Get(ctx context.Context, key string) (*model.IdempotencyRecord, error)
	Save(ctx context.Context, rec *model.IdempotencyRecord) error
}

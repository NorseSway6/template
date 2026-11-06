package idempotency

import (
	"context"

	"github.com/NorseSway6/template.git/internal/model"
)

type store interface {
	Claim(ctx context.Context, key, requestHash string) (bool, *model.IdempotencyRecord, error)
	Complete(ctx context.Context, rec *model.IdempotencyRecord) error
	Release(ctx context.Context, key, requestHash string) error
}

type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

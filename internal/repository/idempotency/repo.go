package idempotency

import (
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

const idempotencyKeyColumn = "key"

// Repository представляет репозиторий для работы с записями идемпотентности.
type Repository struct {
	pool         *pgxpool.Pool
	sq           squirrel.StatementBuilderType
	queryTimeout time.Duration
	ttl          time.Duration
}

// New создает новый экземпляр репозитория идемпотентности.
func New(pool *pgxpool.Pool, queryTimeout, ttl time.Duration) *Repository {
	return &Repository{
		pool:         pool,
		sq:           squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		queryTimeout: queryTimeout,
		ttl:          ttl,
	}
}

package idempotency

import (
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository представляет репозиторий для работы с записями идемпотентности.
type Repository struct {
	pool *pgxpool.Pool
	sq   squirrel.StatementBuilderType
}

// New создает новый экземпляр репозитория идемпотентности.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
		sq:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

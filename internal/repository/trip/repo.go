package trip

import (
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository реализует работу с таблицами trips и trip_status_history.
type Repository struct {
	pool *pgxpool.Pool
	sq   squirrel.StatementBuilderType
}

// New создает новый экземпляр Repository.
func New(db *pgxpool.Pool) *Repository {
	return &Repository{
		pool: db,
		sq:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

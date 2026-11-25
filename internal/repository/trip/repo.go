package trip

import (
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

// statusColumn представляет имя колонки в таблице trips, которая хранит статус поездки.
const statusColumn = "status"

// Repository реализует работу с таблицами trips и trip_status_history.
type Repository struct {
	pool         *pgxpool.Pool
	sq           squirrel.StatementBuilderType
	queryTimeout time.Duration
}

// New создает новый экземпляр Repository.
func New(db *pgxpool.Pool, queryTimeout time.Duration) *Repository {
	return &Repository{
		pool:         db,
		sq:           squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		queryTimeout: queryTimeout,
	}
}

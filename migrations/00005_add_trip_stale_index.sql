-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY trips_active_stale_idx
    ON trips (COALESCE(last_position_at, started_at))
    WHERE status = 'active';
-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
-- +goose NO TRANSACTION
DROP INDEX CONCURRENTLY IF EXISTS trips_active_stale_idx;

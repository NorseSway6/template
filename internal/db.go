package internal

import (
	"context"
	"fmt"

	"github.com/NorseSway6/template.git/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool создает пулл соединений с базой.
func NewPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error){
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database url: %w", err)
	}
	poolCfg.MaxConns = cfg.DatabaseMaxConns
	poolCfg.MinConns = cfg.DatabaseMinConns
	poolCfg.MaxConnLifetime = cfg.DatabaseMaxConnLifetime
	poolCfg.ConnConfig.ConnectTimeout = cfg.DatabaseConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create db pool: %w", err)
	}
	
	pingCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database is unreachable: %w", err)
	}

	return pool, nil
}

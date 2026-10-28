package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NorseSway6/template.git/internal/config"
	httpserver "github.com/NorseSway6/template.git/internal/http"
	httpidem "github.com/NorseSway6/template.git/internal/http/idempotency"
	"github.com/NorseSway6/template.git/internal/http/middleware"
	createhandler "github.com/NorseSway6/template.git/internal/http/trip/create"
	finishhandler "github.com/NorseSway6/template.git/internal/http/trip/finish"
	gethandler "github.com/NorseSway6/template.git/internal/http/trip/get"
	"github.com/NorseSway6/template.git/internal/infra/postgres"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	"github.com/NorseSway6/template.git/internal/infra/telemetry"
	idemrepo "github.com/NorseSway6/template.git/internal/repository/idempotency"
	triprepo "github.com/NorseSway6/template.git/internal/repository/trip"
	createuc "github.com/NorseSway6/template.git/internal/usecase/trip/create"
	finishuc "github.com/NorseSway6/template.git/internal/usecase/trip/finish"
	getuc "github.com/NorseSway6/template.git/internal/usecase/trip/get"
)

// Container собирает зависимости сервиса и управляет его жизненным циклом.
type Container struct {
	cfg          config.Config
	pool         *pgxpool.Pool
	server       *http.Server
	shutdownOTel func(context.Context) error
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger, shutdownOTel func(context.Context) error) (*Container, error) {
	// инфраструктура
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}
	tx := transaction.New(pool)

	// телеметрия
	httpMetrics, err := middleware.NewMetrics()
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create HTTP metrics: %w", err)
	}
	tripMetrics, err := telemetry.NewTripMetrics()
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create trip metrics: %w", err)
	}

	// репозитории
	tripRepo := triprepo.New(pool)
	idemRepo := idemrepo.New(pool)

	now := func() time.Time { return time.Now().UTC() }

	// usecase
	createUC := createuc.New(tripRepo, tx, now, uuid.NewString, tripMetrics, logger)
	getUC := getuc.New(tripRepo)
	finishUC := finishuc.New(tripRepo, tx, now, tripMetrics, logger)

	// HTTP
	srv := httpserver.NewServer(
		createhandler.New(createUC),
		gethandler.New(getUC),
		finishhandler.New(finishUC),
		httpidem.New(idemRepo),
		pool,
		logger,
		httpMetrics,
	)

	return &Container{
		cfg:  cfg,
		pool: pool,
		server: &http.Server{
			Addr:              cfg.HttpAddr,
			Handler:           srv.Handler(),
			ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
			ReadTimeout:       cfg.HTTPReadTimeout,
			WriteTimeout:      cfg.HTTPWriteTimeout,
			IdleTimeout:       cfg.HTTPIdleTimeout,
		},
		shutdownOTel: shutdownOTel,
	}, nil
}

// Run запускает сервер и ждет отмены ctx, затем делает graceful shutdown.
func (c *Container) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := c.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server listen error: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), c.cfg.ShutdownTimeout)
	defer cancel()

	return c.server.Shutdown(shutdownCtx)
}

// Close освобождает ресурсы.
func (c *Container) Close() error {
	c.pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.TelemetryShutdownTimeout)
	defer cancel()

	return c.shutdownOTel(ctx)
}

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
	pushgateway "github.com/NorseSway6/template.git/internal/gateway/push"
	httpserver "github.com/NorseSway6/template.git/internal/http"
	httpidem "github.com/NorseSway6/template.git/internal/http/idempotency"
	"github.com/NorseSway6/template.git/internal/http/middleware"
	positioncreatehandler "github.com/NorseSway6/template.git/internal/http/positions/create"
	positiongethandler "github.com/NorseSway6/template.git/internal/http/positions/get"
	createhandler "github.com/NorseSway6/template.git/internal/http/trip/create"
	finishhandler "github.com/NorseSway6/template.git/internal/http/trip/finish"
	gethandler "github.com/NorseSway6/template.git/internal/http/trip/get"
	"github.com/NorseSway6/template.git/internal/infra/postgres"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	"github.com/NorseSway6/template.git/internal/infra/telemetry"
	idemrepo "github.com/NorseSway6/template.git/internal/repository/idempotency"
	triprepo "github.com/NorseSway6/template.git/internal/repository/trip"
	positioncreateuc "github.com/NorseSway6/template.git/internal/usecase/positions/create"
	positiongetuc "github.com/NorseSway6/template.git/internal/usecase/positions/get"
	createuc "github.com/NorseSway6/template.git/internal/usecase/trip/create"
	finishuc "github.com/NorseSway6/template.git/internal/usecase/trip/finish"
	getuc "github.com/NorseSway6/template.git/internal/usecase/trip/get"
	"github.com/NorseSway6/template.git/internal/worker/positions"
)

// Container собирает зависимости сервиса и управляет его жизненным циклом.
type Container struct {
	cfg             config.Config
	baseCtx         context.Context
	shutdownCtx     context.Context
	cancelShutdown  context.CancelFunc
	pool            *pgxpool.Pool
	server          *http.Server
	positionQueue   *positions.Queue
	positionScanner *positions.Scanner
	positionPool    *positions.Pool
	pushClient      *pushgateway.Client
	positionMetrics *telemetry.PositionMetrics
	shutdownOTel    func(context.Context) error
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger, shutdownOTel func(context.Context) error) (*Container, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// инфраструктура
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}
	tx := transaction.New(pool, cfg.DatabaseQueryTimeout)

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
	positionMetrics, err := telemetry.NewPositionMetrics()
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create position metrics: %w", err)
	}

	// репозитории
	tripRepo := triprepo.New(pool, cfg.DatabaseQueryTimeout)
	idemRepo := idemrepo.New(pool, cfg.DatabaseQueryTimeout, cfg.DatabaseIdempotencyTTL)

	positionQueue, err := positions.NewQueue(int(cfg.PositionQueueSize), positionMetrics)
	if err != nil {
		pool.Close()
		_ = positionMetrics.Shutdown()
		return nil, fmt.Errorf("create position queue: %w", err)
	}
	pushClient, err := pushgateway.New(cfg.PushHTTPUrl, cfg.PushRequestTimeout)
	if err != nil {
		positionQueue.Close()
		pool.Close()
		_ = positionMetrics.Shutdown()
		return nil, fmt.Errorf("create push service client: %w", err)
	}
	positionPool, err := positions.NewPool(
		positionQueue, pushClient, int(cfg.PositionWorkers),
		cfg.PushRequestTimeout, positionMetrics, logger, nil,
	)
	if err != nil {
		pushClient.Close()
		positionQueue.Close()
		pool.Close()
		_ = positionMetrics.Shutdown()
		return nil, fmt.Errorf("create position worker pool: %w", err)
	}
	ticker, err := positions.NewTicker(cfg.PositionCheckInterval)
	if err != nil {
		pushClient.Close()
		positionQueue.Close()
		pool.Close()
		_ = positionMetrics.Shutdown()
		return nil, fmt.Errorf("create position scanner ticker: %w", err)
	}
	positionScanner, err := positions.NewScanner(
		tripRepo, positionQueue, ticker, cfg.PositionStaleAfter, positionMetrics, logger, nil,
	)
	if err != nil {
		ticker.Stop()
		pushClient.Close()
		positionQueue.Close()
		pool.Close()
		_ = positionMetrics.Shutdown()
		return nil, fmt.Errorf("create position scanner: %w", err)
	}

	now := func() time.Time { return time.Now().UTC() }

	// usecase
	createUC := createuc.New(tripRepo, tx, now, uuid.NewString, tripMetrics, logger)
	getUC := getuc.New(tripRepo)
	finishUC := finishuc.New(tripRepo, tx, now, tripMetrics, logger)
	createPositionUC := positioncreateuc.New(tripRepo, tx, now, positionMetrics)
	getPositionsUC := positiongetuc.New(tripRepo)

	// HTTP
	srv := httpserver.NewServer(
		createhandler.New(createUC),
		positioncreatehandler.New(createPositionUC, positionQueue),
		gethandler.New(getUC),
		positiongethandler.New(getPositionsUC),
		finishhandler.New(finishUC),
		httpidem.New(idemRepo, tx),
		pool,
		logger,
		httpMetrics,
		cfg.DatabaseQueryTimeout,
	)

	return &Container{
		cfg:             cfg,
		baseCtx:         context.WithoutCancel(ctx),
		pool:            pool,
		positionQueue:   positionQueue,
		positionScanner: positionScanner,
		positionPool:    positionPool,
		pushClient:      pushClient,
		positionMetrics: positionMetrics,
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
	serviceCtx := context.WithoutCancel(ctx)
	scannerCtx, cancelScanner := context.WithCancel(serviceCtx)
	defer cancelScanner()
	workersCtx, cancelWorkers := context.WithCancel(serviceCtx)
	defer cancelWorkers()

	scannerDone := make(chan struct{})
	go func() {
		defer close(scannerDone)
		c.positionScanner.Run(scannerCtx)
	}()
	poolDone := make(chan struct{})
	go func() {
		defer close(poolDone)
		c.positionPool.Run(workersCtx)
	}()

	errCh := make(chan error, 1)
	go func() {
		if err := c.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var runErr error
	select {
	case err := <-errCh:
		runErr = fmt.Errorf("server listen error: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(serviceCtx, c.cfg.ShutdownTimeout)
	c.shutdownCtx = shutdownCtx
	c.cancelShutdown = cancel

	shutdownErr := c.server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, c.server.Close())
	}

	cancelScanner()
	var scannerErr error
	select {
	case <-scannerDone:
	case <-shutdownCtx.Done():
		scannerErr = fmt.Errorf("stop position scanner: %w", shutdownCtx.Err())
	}
	c.positionScanner.Stop()

	c.positionQueue.Close()
	var poolErr error
	if shutdownCtx.Err() != nil {
		poolErr = fmt.Errorf("drain position worker pool: %w", shutdownCtx.Err())
		cancelWorkers()
	} else {
		select {
		case <-poolDone:
		case <-shutdownCtx.Done():
			poolErr = fmt.Errorf("drain position worker pool: %w", shutdownCtx.Err())
			cancelWorkers()
		}
	}

	if shutdownCtx.Err() != nil && poolErr == nil {
		poolErr = fmt.Errorf("drain position worker pool: %w", shutdownCtx.Err())
	}

	return errors.Join(runErr, shutdownErr, scannerErr, poolErr)
}

// Close освобождает ресурсы.
func (c *Container) Close() error {
	if c.shutdownCtx == nil {
		c.shutdownCtx, c.cancelShutdown = context.WithTimeout(c.baseCtx, c.cfg.ShutdownTimeout)
	}
	ctx := c.shutdownCtx
	c.positionScanner.Stop()
	c.positionQueue.Close()
	c.pushClient.Close()
	defer c.cancelShutdown()
	if ctx.Err() != nil {
		err := fmt.Errorf("shutdown deadline exceeded; forcing process exit: %w", ctx.Err())
		return errors.Join(err, c.positionMetrics.Shutdown(), c.shutdownOTel(ctx))
	}

	poolClosed := make(chan struct{})
	go func() {
		c.pool.Close()
		close(poolClosed)
	}()
	select {
	case <-poolClosed:
	case <-ctx.Done():
		err := fmt.Errorf("close database pool: %w", ctx.Err())
		return errors.Join(err, c.positionMetrics.Shutdown(), c.shutdownOTel(ctx))
	}

	return errors.Join(c.positionMetrics.Shutdown(), c.shutdownOTel(ctx))
}

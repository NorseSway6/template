package container

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NorseSway6/template.git/internal/config"
	httpserver "github.com/NorseSway6/template.git/internal/http"
	httpidem "github.com/NorseSway6/template.git/internal/http/idempotency"
	createhandler "github.com/NorseSway6/template.git/internal/http/trip/create"
	finishhandler "github.com/NorseSway6/template.git/internal/http/trip/finish"
	gethandler "github.com/NorseSway6/template.git/internal/http/trip/get"
	"github.com/NorseSway6/template.git/internal/infra/postgres"
	"github.com/NorseSway6/template.git/internal/infra/postgres/transaction"
	idemrepo "github.com/NorseSway6/template.git/internal/repository/idempotency"
	triprepo "github.com/NorseSway6/template.git/internal/repository/trip"
	createuc "github.com/NorseSway6/template.git/internal/usecase/trip/create"
	finishuc "github.com/NorseSway6/template.git/internal/usecase/trip/finish"
	getuc "github.com/NorseSway6/template.git/internal/usecase/trip/get"
)

// Container собирает зависимости сервиса и управляет его жизненным циклом.
type Container struct {
	cfg    config.Config
	pool   *pgxpool.Pool
	server *http.Server
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	// инфраструктура
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}
	tx := transaction.New(pool)

	// репозитории
	tripRepo := triprepo.New(pool)
	idemRepo := idemrepo.New(pool)

	now := func() time.Time { return time.Now().UTC() }

	// usecase
	createUC := createuc.New(tripRepo, tx, now, uuid.NewString)
	getUC := getuc.New(tripRepo)
	finishUC := finishuc.New(tripRepo, tx, now)

	// HTTP
	srv := httpserver.NewServer(
		createhandler.New(createUC),
		gethandler.New(getUC),
		finishhandler.New(finishUC),
		httpidem.New(idemRepo),
		pool,
	)

	return &Container{
		cfg:  cfg,
		pool: pool,
		server: &http.Server{
			Addr:              cfg.HttpAddr,
			Handler:           srv.Handler(),
			ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
	}, nil
}

// Run запускает сервер и ждет отмены ctx, затем делает graceful shutdown.
func (c *Container) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", c.cfg.HttpAddr)
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
func (c *Container) Close() {
	c.pool.Close()
}

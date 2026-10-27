package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/NorseSway6/template.git/internal/config"
	"github.com/NorseSway6/template.git/internal/container"
	"github.com/NorseSway6/template.git/internal/infra/telemetry"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger, shutdownOTel, err := telemetry.Setup(ctx, "trip-service")
	if err != nil {
		if logger != nil {
			logger.ErrorContext(ctx, "telemetry setup failed", "error", err)
		}
		return err
	}

	cfg, err := config.LoadAndValidateConfig()
	if err != nil {
		logger.ErrorContext(ctx, "configuration load failed", "error", err)
		return errors.Join(err, shutdownOTel(ctx))
	}

	c, err := container.New(ctx, cfg, logger, shutdownOTel)
	if err != nil {
		logger.ErrorContext(ctx, "service initialization failed", "error", err)
		return errors.Join(err, shutdownOTel(ctx))
	}

	runErr := c.Run(ctx)
	closeErr := c.Close()
	if runErr != nil {
		logger.ErrorContext(ctx, "HTTP server stopped unexpectedly", "error", runErr)
	}
	if closeErr != nil {
		logger.ErrorContext(ctx, "service shutdown failed", "error", closeErr)
	}
	return errors.Join(runErr, closeErr)
}

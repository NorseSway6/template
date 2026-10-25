package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/NorseSway6/template.git/internal/config"
	"github.com/NorseSway6/template.git/internal/container"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.LoadAndValidateConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	c, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build container: %w", err)
	}
	defer c.Close()

	return c.Run(ctx)
}

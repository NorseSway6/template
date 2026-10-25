package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/NorseSway6/template.git/internal/config"
	"github.com/NorseSway6/template.git/internal/container"
)

func main() {
	cfg, err := config.LoadAndValidateConfig()
	if err != nil {
		log.Fatal("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	c, err := container.New(ctx, cfg)
	if err != nil {
		log.Fatal("build container: %w", err)
	}
	defer c.Close()

	c.Run(ctx)
}

package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/NorseSway6/template.git/internal"
	"github.com/NorseSway6/template.git/internal/config"
)

func main() {
	// загрузка конфигурации
	cfg, err := config.LoadAndValidateConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	// подключение к бд
	db, err := internal.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	tripRepo := internal.NewTripRepository(db, cfg.DatabaseQueryTimeout)
	idempRepo := internal.NewIdempotencyRepository(db, cfg.DatabaseQueryTimeout)
	txManager := internal.NewTxManager(db, cfg.DatabaseQueryTimeout)

	// инициализация сервера
	srv := internal.NewServer(cfg, db, tripRepo, txManager, idempRepo)

	// запуск сервера
	go func() {
		log.Printf("listening on %s", cfg.HttpAddr)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server listen error: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	shutdownErr := srv.Shutdown(shutdownCtx)
	cancel()
	if shutdownErr != nil {
		log.Printf("graceful shutdown failed: %v", shutdownErr)
		if err := srv.Close(); err != nil {
			log.Printf("forced HTTP server close failed: %v", err)
		}
		log.Fatalf("exiting without waiting for database pool connections")
	}

	db.Close()
}

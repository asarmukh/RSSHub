package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"rsshub/adapter/postgres"
	"rsshub/adapter/rss"
	"rsshub/app"
	"rsshub/cli/control"
	"rsshub/internal/config"
	"rsshub/internal/db"
)

func Fetch(args []string) error {
	cfg := config.Load()

	listener, err := control.TryListen(cfg.ControlAddr)
	if err != nil {
		if errors.Is(err, control.ErrAlreadyRunning) {
			fmt.Println("Background process is already running")
			return err
		}
		return fmt.Errorf("failed to start control server: %w", err)
	}
	defer listener.Close()

	database, err := db.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repo := postgres.New(database)
	if err := repo.Ensure(context.Background()); err != nil {
		return fmt.Errorf("db ensure failed: %w", err)
	}

	fetcher := rss.NewHTTPFetcher()
	agg := app.NewAggregator(repo, fetcher, cfg.DefaultInterval, cfg.DefaultWorkers)
	ctrl := control.NewServer(agg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := http.Serve(listener, ctrl); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("control server error: %v", err)
		}
	}()

	if err := agg.Start(ctx); err != nil {
		return fmt.Errorf("failed to start aggregator: %w", err)
	}

	fmt.Printf("The background process for fetching feeds has started (interval = %s, workers = %d)\n", cfg.DefaultInterval.String(), cfg.DefaultWorkers)

	<-ctx.Done()

	// Stop gracefully
	if err := agg.Stop(); err != nil {
		fmt.Printf("Error during shutdown: %v\n", err)
	} else {
		fmt.Println("Graceful shutdown: aggregator stopped")
	}

	return nil
}

package cmd

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"rsshub/adapter/postgres"
	"rsshub/internal/config"
	"rsshub/internal/db"
)

func Delete(args []string) error {
	fset := flag.NewFlagSet("delete", flag.ContinueOnError)
	var name string
	fset.StringVar(&name, "name", "", "feed name")
	if err := fset.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("--name is required")
	}

	cfg := config.Load()
	database, err := db.OpenDB(cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	repo := postgres.New(database)
	if err := repo.Ensure(context.Background()); err != nil {
		return err
	}

	rows, err := repo.DeleteFeed(context.Background(), name)
	if err != nil {
		return fmt.Errorf("could not delete feed %q: %w", name, err)
	}

	if rows == 0 {
		return fmt.Errorf("feed %q not found", name)
	}

	fmt.Printf("Feed %q deleted successfully\n", name)
	return nil
}

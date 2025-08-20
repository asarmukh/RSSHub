package cmd

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"rsshub/adapter/postgres"
	"rsshub/internal/config"
	"rsshub/internal/db"
)

func Add(args []string) error {
	fset := flag.NewFlagSet("add", flag.ContinueOnError)
	var name string
	var feedURL string
	fset.StringVar(&name, "name", "", "feed name")
	fset.StringVar(&feedURL, "url", "", "feed URL")
	if err := fset.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(name) == "" || strings.TrimSpace(feedURL) == "" {
		return fmt.Errorf("both --name and --url are required")
	}

	if _, err := url.ParseRequestURI(feedURL); err != nil {
		return fmt.Errorf("invalid feed URL: %w", err)
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

	if err := repo.AddFeed(context.Background(), name, feedURL); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return fmt.Errorf("feed %q already exists", name)
		}
		return fmt.Errorf("could not add feed: %w", err)
	}

	fmt.Printf("Feed %q added successfully (%s)\n", name, feedURL)
	return nil
}

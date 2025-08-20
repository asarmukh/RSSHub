package cmd

import (
	"context"
	"flag"
	"fmt"

	"rsshub/adapter/postgres"
	"rsshub/internal/config"
	"rsshub/internal/db"
)

func List(args []string) error {
	fset := flag.NewFlagSet("list", flag.ContinueOnError)
	var num int
	fset.IntVar(&num, "num", 0, "limit number of feeds (0 = all)")
	if err := fset.Parse(args); err != nil {
		return err
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

	feeds, err := repo.ListFeeds(context.Background(), num)
	if err != nil {
		return fmt.Errorf("could not list feeds: %w", err)
	}

	if len(feeds) == 0 {
		fmt.Println("No feeds available")
		return nil
	}

	fmt.Println("Available RSS Feeds\n")
	for i, f := range feeds {
		fmt.Printf("%d. %s\n   URL: %s\n   Added: %s\n\n",
			i+1,
			f.Name,
			f.URL,
			f.CreatedAt.Format("2006-01-02 15:04"),
		)
	}
	return nil
}

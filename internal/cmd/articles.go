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

func Articles(args []string) error {
	fset := flag.NewFlagSet("articles", flag.ContinueOnError)
	var feedName string
	var num int
	fset.StringVar(&feedName, "feed-name", "", "feed name")
	fset.IntVar(&num, "num", 3, "number of articles")
	if err := fset.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(feedName) == "" {
		return fmt.Errorf("--feed-name is required")
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

	feed, err := repo.GetFeedByName(context.Background(), feedName)
	if err != nil {
		return fmt.Errorf("feed %q not found", feedName)
	}

	arts, err := repo.ListArticlesByFeed(context.Background(), feed.ID, num)
	if err != nil {
		return fmt.Errorf("could not fetch articles for %q: %w", feedName, err)
	}

	if len(arts) == 0 {
		fmt.Printf("No articles found for feed %q\n", feedName)
		return nil
	}

	fmt.Printf("Articles from feed: %s\n\n", feed.Name)
	for i, a := range arts {
		fmt.Printf("%d. [%s] %s\n   %s\n\n",
			i+1,
			a.PublishedAt.Format("2006-01-02"),
			a.Title,
			a.Link,
		)
	}
	return nil
}

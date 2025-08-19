package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"rsshub/adapter/postgres"
	rss "rsshub/adapter/rss"
	"rsshub/app"
	"rsshub/cli/control"
	"rsshub/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "--help", "-h", "help":
		printHelp()
		return
	case "fetch":
		if err := cmdFetch(args); err != nil {
			log.Printf("%v", err)
			os.Exit(1)
		}
	case "add":
		if err := cmdAdd(args); err != nil {
			log.Printf("%v", err)
			os.Exit(1)
		}
	case "list":
		if err := cmdList(args); err != nil {
			log.Printf("%v", err)
			os.Exit(1)
		}
	case "delete":
		if err := cmdDelete(args); err != nil {
			log.Printf("%v", err)
			os.Exit(1)
		}
	case "articles":
		if err := cmdArticles(args); err != nil {
			log.Printf("%v", err)
			os.Exit(1)
		}
	case "set-interval":
		if err := cmdSetInterval(args); err != nil {
			log.Printf("%v", err)
			os.Exit(1)
		}
	case "set-workers":
		if err := cmdSetWorkers(args); err != nil {
			log.Printf("%v", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("unknown command: %s\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`Usage:
  rsshub COMMAND [OPTIONS]

Common Commands:
   add             add new RSS feed
   set-interval    set RSS fetch interval
   set-workers     set number of workers
   list            list available RSS feeds
   delete          delete RSS feed
   articles        show latest articles
   fetch           starts the background process that periodically fetches and processes RSS feeds using a worker pool
`)
}

func cmdFetch(args []string) error {
	cfg := config.Load()

	listener, err := control.TryListen(cfg.ControlAddr)
	if err != nil {
		if errors.Is(err, control.ErrAlreadyRunning) {
			fmt.Println("Background process is already running")
			return err
		}
		return err
	}
	defer listener.Close()

	database, err := openDB(cfg)
	if err != nil {
		return err
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
		_ = http.Serve(listener, ctrl)
	}()

	if err := agg.Start(ctx); err != nil {
		return err
	}

	fmt.Printf("The background process for fetching feeds has started (interval = %s, workers = %d)\n", cfg.DefaultInterval.String(), cfg.DefaultWorkers)

	<-ctx.Done()
	_ = agg.Stop()
	fmt.Println("Graceful shutdown: aggregator stopped")
	return nil
}

func cmdAdd(args []string) error {
	fset := flag.NewFlagSet("add", flag.ContinueOnError)
	var name string
	var url string
	fset.StringVar(&name, "name", "", "feed name")
	fset.StringVar(&url, "url", "", "feed URL")
	if err := fset.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" || strings.TrimSpace(url) == "" {
		return fmt.Errorf("both --name and --url are required")
	}
	cfg := config.Load()
	database, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer database.Close()
	repo := postgres.New(database)
	if err := repo.Ensure(context.Background()); err != nil {
		return err
	}
	if err := repo.AddFeed(context.Background(), name, url); err != nil {
		return err
	}
	return nil
}

func cmdList(args []string) error {
	fset := flag.NewFlagSet("list", flag.ContinueOnError)
	var num int
	fset.IntVar(&num, "num", 0, "limit number of feeds (0 = all)")
	if err := fset.Parse(args); err != nil {
		return err
	}
	cfg := config.Load()
	database, err := openDB(cfg)
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
		return err
	}
	fmt.Println("# Available RSS Feeds\n")
	for i, f := range feeds {
		fmt.Printf("%d. Name: %s\n   URL: %s\n   Added: %s\n\n", i+1, f.Name, f.URL, f.CreatedAt.Format("2006-01-02 15:04"))
	}
	return nil
}

func cmdDelete(args []string) error {
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
	database, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer database.Close()
	repo := postgres.New(database)
	if err := repo.Ensure(context.Background()); err != nil {
		return err
	}
	return repo.DeleteFeed(context.Background(), name)
}

func cmdArticles(args []string) error {
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
	database, err := openDB(cfg)
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
		return err
	}
	arts, err := repo.ListArticlesByFeed(context.Background(), feed.ID, num)
	if err != nil {
		return err
	}
	fmt.Printf("Feed: %s\n\n", feed.Name)
	for i, a := range arts {
		fmt.Printf("%d. [%s] %s\n   %s\n\n", i+1, a.PublishedAt.Format("2006-01-02"), a.Title, a.Link)
	}
	return nil
}

func cmdSetInterval(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: rsshub set-interval DURATION (e.g., 2m)")
	}
	d, err := time.ParseDuration(args[0])
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	c := control.NewClient(config.Load().ControlAddr)
	old, err := c.SetInterval(d)
	if err != nil {
		return err
	}
	fmt.Printf("Interval of fetching feeds changed from %s to %s\n", old.String(), d.String())
	return nil
}

func cmdSetWorkers(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: rsshub set-workers COUNT")
	}
	var n int
	_, err := fmt.Sscanf(args[0], "%d", &n)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid workers count: %v", args[0])
	}
	c := control.NewClient(config.Load().ControlAddr)
	old, err := c.SetWorkers(n)
	if err != nil {
		return err
	}
	fmt.Printf("Number of workers changed from %d to %d\n", old, n)
	return nil
}

func openDB(cfg config.Config) (*sql.DB, error) {
	pgURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.PGUser, cfg.PGPassword, cfg.PGHost, cfg.PGPort, cfg.PGDatabase,
	)
	dbConn, err := sql.Open("postgres", pgURL)
	if err != nil {
		return nil, err
	}
	dbConn.SetMaxOpenConns(10)
	dbConn.SetMaxIdleConns(10)
	dbConn.SetConnMaxLifetime(30 * time.Minute)
	if err := dbConn.Ping(); err != nil {
		return nil, err
	}
	return dbConn, nil
}

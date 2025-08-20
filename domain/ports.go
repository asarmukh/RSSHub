package domain

import (
	"context"
	"time"
)

// FeedRepository is the persistence port for feeds and articles.
type FeedRepository interface {
	Ensure(ctx context.Context) error
	AddFeed(ctx context.Context, name, url string) error
	DeleteFeed(ctx context.Context, name string) (int64, error)
	ListFeeds(ctx context.Context, limit int) ([]Feed, error)
	GetFeedByName(ctx context.Context, name string) (Feed, error)
	ListArticlesByFeed(ctx context.Context, feedID string, limit int) ([]Article, error)
	UpsertArticle(ctx context.Context, a Article) error
	GetStaleFeeds(ctx context.Context, limit int) ([]Feed, error)
	MarkFeedPolled(ctx context.Context, feedID string) error
}

// RSSFetcher fetches and parses RSS feeds.
type RSSFetcher interface {
	Fetch(ctx context.Context, feedURL string) ([]FetchedItem, error)
}

// Aggregator exposes application-level controls for background processing.
type Aggregator interface {
	Start(ctx context.Context) error
	Stop() error

	SetInterval(d time.Duration)
	Resize(workers int) error
	CurrentInterval() time.Duration
	CurrentWorkers() int
}

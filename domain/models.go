package domain

import "time"

type Feed struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	URL       string
}

type Article struct {
	ID          string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Title       string
	Link        string
	PublishedAt time.Time
	Description string
	FeedID      string
}

// FetchedItem is a simplified representation returned by RSS fetchers.
type FetchedItem struct {
	Title       string
	Link        string
	Description string
	PublishedAt time.Time
}

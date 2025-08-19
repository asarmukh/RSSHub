package db

import (
	"database/sql"
	"time"
)

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

func Ensure(db *sql.DB) error {
	_, err := db.Exec(`
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE IF NOT EXISTS feeds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    name TEXT UNIQUE NOT NULL,
    url TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    title TEXT NOT NULL,
    link TEXT NOT NULL,
    published_at TIMESTAMP NOT NULL,
    description TEXT NOT NULL,
    feed_id UUID NOT NULL REFERENCES feeds(id),
    UNIQUE (feed_id, link)
);
`)
	return err
}



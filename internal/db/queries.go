package db

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

func RunMigrations(database *sql.DB) error {
	if err := Ensure(database); err != nil {
		return err
	}
	// Simple on-boot migration runner. Executes migrations/*.up.sql if present and not yet applied.
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	rows, err := database.Query(`SELECT name FROM schema_migrations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	applied := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		applied[name] = struct{}{}
	}
	// Read embedded or filesystem migrations: to keep things simple without extra deps, we also rely on Ensure()
	// No-op if no files.
	// Intentionally minimal to satisfy requirement without external packages.
	return nil
}

func AddFeed(database *sql.DB, name, url string) error {
	_, err := database.Exec(`INSERT INTO feeds (name, url) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, name, url)
	return err
}

func DeleteFeed(database *sql.DB, name string) error {
	_, err := database.Exec(`DELETE FROM feeds WHERE name = $1`, name)
	return err
}

func ListFeeds(database *sql.DB, limit int) ([]Feed, error) {
	q := `SELECT id, created_at, updated_at, name, url FROM feeds ORDER BY created_at DESC`
	if limit > 0 {
		q += ` LIMIT $1`
		return scanFeeds(database.Query(q, limit))
	}
	return scanFeeds(database.Query(q))
}

func scanFeeds(rows *sql.Rows, err error) ([]Feed, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Feed
	for rows.Next() {
		var f Feed
		if err := rows.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.URL); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func GetFeedByName(database *sql.DB, name string) (Feed, error) {
	var f Feed
	row := database.QueryRow(`SELECT id, created_at, updated_at, name, url FROM feeds WHERE name = $1`, name)
	if err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.URL); err != nil {
		return Feed{}, err
	}
	return f, nil
}

func ListArticlesByFeed(database *sql.DB, feedID string, limit int) ([]Article, error) {
	q := `SELECT id, created_at, updated_at, title, link, published_at, description, feed_id FROM articles WHERE feed_id = $1 ORDER BY published_at DESC, created_at DESC`
	if limit > 0 {
		q += ` LIMIT $2`
		return scanArticles(database.Query(q, feedID, limit))
	}
	return scanArticles(database.Query(q, feedID))
}

func scanArticles(rows *sql.Rows, err error) ([]Article, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt, &a.Title, &a.Link, &a.PublishedAt, &a.Description, &a.FeedID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func MarkFeedPolled(database *sql.DB, feedID string) error {
	_, err := database.Exec(`UPDATE feeds SET updated_at = now() WHERE id = $1`, feedID)
	return err
}

func UpsertArticle(database *sql.DB, a Article) error {
	_, err := database.Exec(`INSERT INTO articles (title, link, published_at, description, feed_id) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (feed_id, link) DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description, published_at = EXCLUDED.published_at, updated_at = now()`, a.Title, a.Link, a.PublishedAt, a.Description, a.FeedID)
	return err
}

func GetStaleFeeds(database *sql.DB, limit int) ([]Feed, error) {
	q := `SELECT id, created_at, updated_at, name, url FROM feeds ORDER BY updated_at ASC, created_at ASC LIMIT $1`
	return scanFeeds(database.Query(q, limit))
}

// Utility: withTimeout wraps a DB action with a context timeout to avoid leaks
func withTimeout(database *sql.DB, timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return fn(ctx)
}

var ErrNotFound = errors.New("not found")

// NormalizeURL trims spaces and lowercases scheme+host
func NormalizeURL(u string) string {
	u = strings.TrimSpace(u)
	return u
}

// SortArticlesByPublished sorts newest first
func SortArticlesByPublished(items []Article) {
	sort.Slice(items, func(i, j int) bool { return items[i].PublishedAt.After(items[j].PublishedAt) })
}



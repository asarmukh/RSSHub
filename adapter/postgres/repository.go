package postgres

import (
	"context"
	"database/sql"
	"time"

	"rsshub/domain"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Ensure(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
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
	if err != nil {
		return err
	}
	_, _ = r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT now())`)
	return nil
}

func (r *Repository) AddFeed(ctx context.Context, name, url string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO feeds (name, url) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, name, url)
	return err
}

func (r *Repository) DeleteFeed(ctx context.Context, name string) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM feeds WHERE name = $1`, name)
	if err != nil {
		return 0, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rows, nil
}

func (r *Repository) ListFeeds(ctx context.Context, limit int) ([]domain.Feed, error) {
	q := `SELECT id, created_at, updated_at, name, url FROM feeds ORDER BY created_at DESC`
	if limit > 0 {
		q += ` LIMIT $1`
		return scanFeeds(r.db.QueryContext(ctx, q, limit))
	}
	return scanFeeds(r.db.QueryContext(ctx, q))
}

func (r *Repository) GetFeedByName(ctx context.Context, name string) (domain.Feed, error) {
	var f domain.Feed
	row := r.db.QueryRowContext(ctx, `SELECT id, created_at, updated_at, name, url FROM feeds WHERE name = $1`, name)
	if err := row.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.URL); err != nil {
		return domain.Feed{}, err
	}
	return f, nil
}

func (r *Repository) ListArticlesByFeed(ctx context.Context, feedID string, limit int) ([]domain.Article, error) {
	q := `SELECT id, created_at, updated_at, title, link, published_at, description, feed_id FROM articles WHERE feed_id = $1 ORDER BY published_at DESC, created_at DESC`
	if limit > 0 {
		q += ` LIMIT $2`
		return scanArticles(r.db.QueryContext(ctx, q, feedID, limit))
	}
	return scanArticles(r.db.QueryContext(ctx, q, feedID))
}

func (r *Repository) UpsertArticle(ctx context.Context, a domain.Article) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO articles (title, link, published_at, description, feed_id) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (feed_id, link) DO UPDATE SET title = EXCLUDED.title, description = EXCLUDED.description, published_at = EXCLUDED.published_at, updated_at = now()`, a.Title, a.Link, a.PublishedAt, a.Description, a.FeedID)
	return err
}

func (r *Repository) GetStaleFeeds(ctx context.Context, limit int) ([]domain.Feed, error) {
	q := `SELECT id, created_at, updated_at, name, url FROM feeds ORDER BY updated_at ASC, created_at ASC LIMIT $1`
	return scanFeeds(r.db.QueryContext(ctx, q, limit))
}

func (r *Repository) MarkFeedPolled(ctx context.Context, feedID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE feeds SET updated_at = now() WHERE id = $1`, feedID)
	return err
}

func scanFeeds(rows *sql.Rows, err error) ([]domain.Feed, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Feed
	for rows.Next() {
		var f domain.Feed
		if err := rows.Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt, &f.Name, &f.URL); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func scanArticles(rows *sql.Rows, err error) ([]domain.Article, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Article
	for rows.Next() {
		var a domain.Article
		if err := rows.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt, &a.Title, &a.Link, &a.PublishedAt, &a.Description, &a.FeedID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// Utility: optional timeout wrapper
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}

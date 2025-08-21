package rss

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"rsshub/domain"
	"time"
)

type HTTPFetcher struct{ client *http.Client }

func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{client: &http.Client{Timeout: 20 * time.Second}}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, feedURL string) ([]domain.FetchedItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, nil
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var rf rssFeed
	if err := xml.Unmarshal(bytes, &rf); err != nil {
		return nil, err
	}
	items := make([]domain.FetchedItem, 0, len(rf.Channel.Item))
	for _, it := range rf.Channel.Item {
		var published time.Time
		if it.PubDate != "" {
			if p, perr := time.Parse(time.RFC1123Z, it.PubDate); perr == nil {
				published = p
			} else if p2, perr2 := time.Parse(time.RFC1123, it.PubDate); perr2 == nil {
				published = p2
			} else {
				published = time.Now()
			}
		} else {
			published = time.Now()
		}
		items = append(items, domain.FetchedItem{
			Title:       it.Title,
			Link:        it.Link,
			Description: it.Description,
			PublishedAt: published,
		})
	}
	return items, nil
}

type rssFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

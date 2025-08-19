package aggregator

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"rsshub/internal/db"
	"sync"
	"time"
)

type Aggregator struct {
	db *sql.DB

	mu             sync.Mutex
	interval       time.Duration
	workers        int
	jobs           chan db.Feed
	ctx            context.Context
	cancel         context.CancelFunc
	tickerStopChan chan struct{}
	started        bool
	workerCancels  []context.CancelFunc
}

func New(database *sql.DB, interval time.Duration, workers int) *Aggregator {
	return &Aggregator{db: database, interval: interval, workers: workers}
}

func (a *Aggregator) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started {
		return errors.New("aggregator already started")
	}
	a.ctx, a.cancel = context.WithCancel(ctx)
	if a.jobs == nil {
		a.jobs = make(chan db.Feed)
	}
	a.tickerStopChan = make(chan struct{})
	a.workerCancels = nil
	startWorkersCount(a, a.workers)
	go a.loop()
	a.started = true
	return nil
}

func (a *Aggregator) Stop() error {
	a.mu.Lock()
	if !a.started {
		a.mu.Unlock()
		return nil
	}
	cancel := a.cancel
	stopCh := a.tickerStopChan
	cancels := append([]context.CancelFunc(nil), a.workerCancels...)
	a.started = false
	a.mu.Unlock()

	close(stopCh)
	cancel()
	for _, c := range cancels {
		c()
	}
	return nil
}

func (a *Aggregator) SetInterval(d time.Duration) { // dynamic ticker change
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.started {
		a.interval = d
		return
	}
	// signal loop to restart ticker by closing old stop chan and replacing it
	close(a.tickerStopChan)
	a.tickerStopChan = make(chan struct{})
	a.interval = d
}

func (a *Aggregator) Resize(workers int) error {
	if workers <= 0 {
		return errors.New("workers must be > 0")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.workers == workers {
		return nil
	}
	if workers > a.workers {
		delta := workers - a.workers
		startWorkersCount(a, delta)
	} else {
		delta := a.workers - workers
		for i := 0; i < delta && len(a.workerCancels) > 0; i++ {
			idx := len(a.workerCancels) - 1
			c := a.workerCancels[idx]
			a.workerCancels = a.workerCancels[:idx]
			c()
		}
	}
	a.workers = workers
	return nil
}

func (a *Aggregator) CurrentInterval() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.interval
}

func (a *Aggregator) CurrentWorkers() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.workers
}

func (a *Aggregator) loop() {
	for {
		a.mu.Lock()
		interval := a.interval
		stopCh := a.tickerStopChan
		jobs := a.jobs
		workers := a.workers
		a.mu.Unlock()

		ticker := time.NewTicker(interval)
		select {
		case <-a.ctx.Done():
			ticker.Stop()
			return
		case <-stopCh:
			ticker.Stop()
			continue
		case <-ticker.C:
			// fallthrough to run fetch now
		}

		// schedule work
		feeds, err := db.GetStaleFeeds(a.db, workers)
		if err == nil {
			for _, f := range feeds {
				select {
				case jobs <- f:
				case <-a.ctx.Done():
					return
				}
			}
		}
	}
}

func startWorkersCount(a *Aggregator, count int) {
	for i := 0; i < count; i++ {
		wctx, cancel := context.WithCancel(a.ctx)
		a.workerCancels = append(a.workerCancels, cancel)
		go worker(wctx, a.db, a.jobs)
	}
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

func worker(ctx context.Context, database *sql.DB, jobs <-chan db.Feed) {
	client := &http.Client{Timeout: 20 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-jobs:
			if !ok {
				return
			}
			processFeed(ctx, client, database, f)
		}
	}
}

func processFeed(ctx context.Context, client *http.Client, database *sql.DB, f db.Feed) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var rf rssFeed
	if err := xml.Unmarshal(bytes, &rf); err != nil {
		return
	}
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
		_ = db.UpsertArticle(database, db.Article{
			Title:       it.Title,
			Link:        it.Link,
			Description: it.Description,
			PublishedAt: published,
			FeedID:      f.ID,
		})
	}
	_ = db.MarkFeedPolled(database, f.ID)
}

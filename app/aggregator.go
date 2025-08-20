package app

import (
	"context"
	"errors"
	"rsshub/domain"
	"sync"
	"time"
)

type AggregatorService struct {
	repo    domain.FeedRepository
	fetcher domain.RSSFetcher

	mu             sync.Mutex
	interval       time.Duration
	workers        int
	jobs           chan domain.Feed
	ctx            context.Context
	cancel         context.CancelFunc
	tickerStopChan chan struct{}
	started        bool
	workerCancels  []context.CancelFunc
}

func NewAggregator(repo domain.FeedRepository, fetcher domain.RSSFetcher, interval time.Duration, workers int) *AggregatorService {
	return &AggregatorService{repo: repo, fetcher: fetcher, interval: interval, workers: workers}
}

func (a *AggregatorService) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started {
		return errors.New("aggregator already started")
	}
	a.ctx, a.cancel = context.WithCancel(ctx)
	if a.jobs == nil {
		a.jobs = make(chan domain.Feed)
	}
	a.tickerStopChan = make(chan struct{})
	a.workerCancels = nil
	startWorkersCount(a, a.workers)
	go a.loop()
	a.started = true
	return nil
}

func (a *AggregatorService) Stop() error {
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

func (a *AggregatorService) SetInterval(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.started {
		a.interval = d
		return
	}
	close(a.tickerStopChan)
	a.tickerStopChan = make(chan struct{})
	a.interval = d
}

func (a *AggregatorService) Resize(workers int) error {
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

func (a *AggregatorService) CurrentInterval() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.interval
}

func (a *AggregatorService) CurrentWorkers() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.workers
}

func (a *AggregatorService) loop() {
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
		}

		feeds, err := a.repo.GetStaleFeeds(a.ctx, workers)
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

func startWorkersCount(a *AggregatorService, count int) {
	for i := 0; i < count; i++ {
		wctx, cancel := context.WithCancel(a.ctx)
		a.workerCancels = append(a.workerCancels, cancel)
		go worker(wctx, a.repo, a.fetcher, a.jobs)
	}
}

func worker(ctx context.Context, repo domain.FeedRepository, fetcher domain.RSSFetcher, jobs <-chan domain.Feed) {
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-jobs:
			if !ok {
				return
			}
			processFeed(ctx, repo, fetcher, f)
		}
	}
}

func processFeed(ctx context.Context, repo domain.FeedRepository, fetcher domain.RSSFetcher, f domain.Feed) {
	items, err := fetcher.Fetch(ctx, f.URL)
	if err != nil {
		return
	}
	for _, it := range items {
		_ = repo.UpsertArticle(ctx, domain.Article{
			Title:       it.Title,
			Link:        it.Link,
			Description: it.Description,
			PublishedAt: it.PublishedAt,
			FeedID:      f.ID,
		})
	}
	_ = repo.MarkFeedPolled(ctx, f.ID)
}

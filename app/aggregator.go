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
	// protect against concurrent calls to start by blocking mutex
	a.mu.Lock()
	defer a.mu.Unlock()

	// prevent starting twice
	if a.started {
		return errors.New("aggregator already started")
	}

	// create a cancellable context that will stop when either
	// the passed-in coxtext is cancelled
	// or we explicitly call a.cancel()
	a.ctx, a.cancel = context.WithCancel(ctx)

	// init jobs channel if it hasnt been created yet
	// this channel is where feed tasks will be send to workers
	if a.jobs == nil {
		a.jobs = make(chan domain.Feed)
	}

	// create a channel to signal when the ticker (periodic fetch loop) should stop
	a.tickerStopChan = make(chan struct{})

	// reset any existing workercancel functions
	a.workerCancels = nil

	// start the configured number of worker goroutines
	startWorkersCount(a, a.workers)

	// start main aggregator loop in background
	// this loop typically handles the periodic scheduling of feed fetches
	go a.loop()

	// mark aggregator as started
	a.started = true

	return nil
}

func (a *AggregatorService) Stop() error {
	// lock mutex to make sure only one goroutine can stop the service at a time
	a.mu.Lock()

	// if aggregator was never started just return
	if !a.started {
		a.mu.Unlock()
		return nil
	}

	// save references we wil need after unlocking
	// cancel: cancels the aggregator main context
	// stopCh: channel to signal the ticker loop to stop
	// cancels: list of cancel functions for all worker goroutines
	cancel := a.cancel
	stopCh := a.tickerStopChan
	cancels := append([]context.CancelFunc(nil), a.workerCancels...)

	// mark aggregator as stopped
	a.started = false

	// unlock early since we dont neeed to hold the lock during shutdown
	a.mu.Unlock()

	// close the stop channel tells the loop goroutine to exit
	close(stopCh)

	// Cancel the aggregators context any operations depending on it will stop
	cancel()

	// call cancel function for each worker
	for _, c := range cancels {
		c()
	}

	return nil
}

func (a *AggregatorService) SetInterval(d time.Duration) {
	// lock to make sure multiple goroutines wont change interval at the same time
	a.mu.Lock()
	defer a.mu.Unlock()

	// if aggregator is to running yet
	//	just update interval value so that when Start() is called it will use the new interval
	if !a.started {
		a.interval = d
		return
	}

	// if aggregator already running
	// close old ticker stop channel
	close(a.tickerStopChan)

	// create a new stop channel for the new ticker
	a.tickerStopChan = make(chan struct{})

	// update interval value to the new durations
	a.interval = d
}

func (a *AggregatorService) Resize(workers int) error {
	// valiidate input must bel greater than 0
	if workers <= 0 {
		return errors.New("workers must be > 0")
	}

	// lock to make sure multiple goroutines wont change workers quantity at the same time
	a.mu.Lock()
	defer a.mu.Unlock()

	// if requested number of workers same as current just return
	if a.workers == workers {
		return nil
	}

	// case 1: we need to increase number of workers
	if workers > a.workers {
		delta := workers - a.workers // how many more workers to start
		startWorkersCount(a, delta)
	} else {
		// case 1: we need to decrease number of workers
		delta := a.workers - workers // how many more workers to stop

		// stop delta workers (but not more than available)
		for i := 0; i < delta && len(a.workerCancels) > 0; i++ {
			// take the last workers cancel function
			idx := len(a.workerCancels) - 1
			c := a.workerCancels[idx]

			// remove it from the list of active worker cancels
			a.workerCancels = a.workerCancels[:idx]

			// Call the cancel function to stop that worker goroutine
			c()
		}
	}

	// update current worker count
	a.workers = workers
	return nil
}

// return current interval
func (a *AggregatorService) CurrentInterval() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.interval
}

// return current quantity of workers
func (a *AggregatorService) CurrentWorkers() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.workers
}

func (a *AggregatorService) loop() {
	for {
		// lock and copy state so we dont need to keep the lock during long operations
		a.mu.Lock()
		interval := a.interval     // how often to fetch feeds
		stopCh := a.tickerStopChan // channel used to signal ticker reser
		jobs := a.jobs             // channel for send feeds to workers
		workers := a.workers       // current number of workers
		a.mu.Unlock()

		// create a new ticker that ticks every `interval`
		ticker := time.NewTicker(interval)

		// wait for  one of the following events
		select {
		case <-a.ctx.Done():
			// if aggregator context is canceled
			// stop ticker and exit the loop graceful shutdown
			ticker.Stop()
			return
		case <-stopCh:
			// if interval was changed (`SetInterval` called)
			// close old ticker and restart loop with new interval
			ticker.Stop()
			continue
		case <-ticker.C:
			// ticker fired time to fetch feeds
		}

		// fetch stale feeds that need updating
		// pass the current number of workers so we dont overload
		feeds, err := a.repo.GetStaleFeeds(a.ctx, workers)
		if err == nil {
			// send each feed to the jibs channel for workers to process
			for _, f := range feeds {
				select {
				case jobs <- f: // hand off work to a worker
				case <-a.ctx.Done():
					// if aggregator is shutting down exit immediately
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

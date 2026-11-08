package checker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/store"
)

type siteSchedule struct {
	site   *store.Site
	ticker *time.Ticker
	stop   chan struct{}
}

// Scheduler coordinates timed checks across all registered sites using per-site tickers.
type Scheduler struct {
	store        store.Store
	engine       *Engine
	pool         *WorkerPool
	syncInterval time.Duration

	mu        sync.RWMutex
	schedules map[uuid.UUID]*siteSchedule
	stopCh    chan struct{}
	stopped   bool
}

// NewScheduler creates a new site check Scheduler.
func NewScheduler(s store.Store, engine *Engine, pool *WorkerPool, syncInterval ...time.Duration) *Scheduler {
	interval := 10 * time.Second
	if len(syncInterval) > 0 && syncInterval[0] > 0 {
		interval = syncInterval[0]
	}

	return &Scheduler{
		store:        s,
		engine:       engine,
		pool:         pool,
		syncInterval: interval,
		schedules:    make(map[uuid.UUID]*siteSchedule),
		stopCh:       make(chan struct{}),
	}
}

// Start begins the scheduling loop and performs an immediate sync of active sites.
func (s *Scheduler) Start(ctx context.Context) error {
	// Initial site sync
	if err := s.SyncSites(ctx); err != nil {
		log.Printf("Warning: initial site sync failed: %v", err)
	}

	go s.syncLoop(ctx)
	return nil
}

func (s *Scheduler) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.SyncSites(ctx)
		}
	}
}

// SyncSites refreshes the active sites from the store, updating tickers as sites change.
func (s *Scheduler) SyncSites(ctx context.Context) error {
	sites, err := s.store.GetAllActiveSites(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil
	}

	activeIDs := make(map[uuid.UUID]bool)

	for _, site := range sites {
		activeIDs[site.ID] = true
		sched, exists := s.schedules[site.ID]

		if !exists {
			// New site registered
			s.startSiteTickerLocked(site)
		} else if sched.site.CheckIntervalSec != site.CheckIntervalSec || sched.site.URL != site.URL {
			// Config changed, restart ticker
			sched.stopTicker()
			s.startSiteTickerLocked(site)
		}
	}

	// Remove deleted or inactive sites
	for id, sched := range s.schedules {
		if !activeIDs[id] {
			sched.stopTicker()
			delete(s.schedules, id)
		}
	}

	return nil
}

func (s *Scheduler) startSiteTickerLocked(site *store.Site) {
	interval := time.Duration(site.CheckIntervalSec) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	sched := &siteSchedule{
		site:   site,
		ticker: time.NewTicker(interval),
		stop:   make(chan struct{}),
	}
	s.schedules[site.ID] = sched

	// Perform an immediate initial check
	s.dispatchCheck(site)

	// Start ticker listener
	go func(sch *siteSchedule) {
		for {
			select {
			case <-sch.stop:
				return
			case <-s.stopCh:
				return
			case <-sch.ticker.C:
				s.dispatchCheck(sch.site)
			}
		}
	}(sched)
}

func (sch *siteSchedule) stopTicker() {
	sch.ticker.Stop()
	select {
	case <-sch.stop:
	default:
		close(sch.stop)
	}
}

func (s *Scheduler) dispatchCheck(site *store.Site) {
	s.pool.Submit(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(site.TimeoutSec+5)*time.Second)
		defer cancel()
		_, _, _ = s.engine.CheckSite(ctx, site)
	})
}

// TriggerCheck immediately enqueues a check for the given site ID.
func (s *Scheduler) TriggerCheck(ctx context.Context, siteID uuid.UUID) error {
	s.mu.RLock()
	sched, ok := s.schedules[siteID]
	s.mu.RUnlock()

	if ok {
		s.dispatchCheck(sched.site)
		return nil
	}

	// If not scheduled, fetch from store
	sites, err := s.store.GetAllActiveSites(ctx)
	if err != nil {
		return err
	}
	for _, site := range sites {
		if site.ID == siteID {
			s.dispatchCheck(site)
			return nil
		}
	}
	return store.ErrSiteNotFound
}

// Stop gracefully shuts down the scheduler, stopping all tickers and waiting for workers.
func (s *Scheduler) Stop(timeout ...time.Duration) bool {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return true
	}
	s.stopped = true
	close(s.stopCh)

	for id, sched := range s.schedules {
		sched.stopTicker()
		delete(s.schedules, id)
	}
	s.mu.Unlock()

	return s.pool.Stop(timeout...)
}

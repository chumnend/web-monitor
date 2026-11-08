package checker_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/checker"
	"web-monitor/internal/store"
)

func TestProbeSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer ts.Close()

	opts := checker.ProbeOptions{
		TimeoutSec:        5,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 299,
	}

	res := checker.Probe(context.Background(), ts.Client(), ts.URL, opts)

	if !res.IsUp {
		t.Fatalf("expected site to be up, error: %v", res.ErrorMessage)
	}
	if res.StatusCode == nil || *res.StatusCode != 200 {
		t.Errorf("expected status code 200, got %v", res.StatusCode)
	}
	if res.ResponseTimeMs == nil || *res.ResponseTimeMs < 0 {
		t.Errorf("expected valid response time, got %v", res.ResponseTimeMs)
	}
	if res.TimingBreakdown == nil {
		t.Fatal("expected timing breakdown to be populated")
	}
}

func TestProbeHttpStatusFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	opts := checker.ProbeOptions{
		TimeoutSec:        5,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 399,
	}

	res := checker.Probe(context.Background(), ts.Client(), ts.URL, opts)

	if res.IsUp {
		t.Fatal("expected site to be down on 500 status")
	}
	if res.StatusCode == nil || *res.StatusCode != 500 {
		t.Errorf("expected status code 500, got %v", res.StatusCode)
	}
	if res.ErrorType == nil || *res.ErrorType != "http_status" {
		t.Errorf("expected error type 'http_status', got %v", res.ErrorType)
	}
}

func TestProbeTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Short timeout of 100ms
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	opts := checker.ProbeOptions{
		TimeoutSec:        1,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 399,
	}

	res := checker.Probe(ctx, ts.Client(), ts.URL, opts)

	if res.IsUp {
		t.Fatal("expected probe to fail on timeout")
	}
	if res.ErrorType == nil || *res.ErrorType != store.ErrorTypeTimeout {
		t.Errorf("expected error type 'timeout', got %v", res.ErrorType)
	}
}

func TestProbeConnectionRefused(t *testing.T) {
	opts := checker.ProbeOptions{
		TimeoutSec:        2,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 399,
	}

	// Unused local port
	res := checker.Probe(context.Background(), http.DefaultClient, "http://127.0.0.1:59999", opts)

	if res.IsUp {
		t.Fatal("expected probe to fail on non-existent port")
	}
	if res.ErrorType == nil || *res.ErrorType != store.ErrorTypeConnectionRefused {
		t.Errorf("expected error type 'connection_refused', got %v", res.ErrorType)
	}
}

func TestProbeWithRetry(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := atomic.AddInt32(&attempts, 1)
		if att == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	site := &store.Site{
		URL:               ts.URL,
		TimeoutSec:        3,
		RetryCount:        2,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 299,
	}

	res := checker.ProbeWithRetry(context.Background(), ts.Client(), site, 10*time.Millisecond)

	if !res.IsUp {
		t.Fatalf("expected probe to succeed on second retry attempt, error: %v", res.ErrorMessage)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestEngineCheckSiteAndTransitions(t *testing.T) {
	var shouldFail int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&shouldFail) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	memStore := store.NewMemoryStore()
	userID := uuid.New()
	site := &store.Site{
		ID:                uuid.New(),
		UserID:            userID,
		URL:               ts.URL,
		NormalizedURL:     "test.example.com",
		TimeoutSec:        2,
		FailureThreshold:  2,
		RetryCount:        0,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 299,
	}
	_, err := memStore.CreateSite(context.Background(), site, nil)
	if err != nil {
		t.Fatalf("failed to create site: %v", err)
	}

	engine := checker.NewEngine(memStore, ts.Client())
	engine.SetRetryBackoff(10 * time.Millisecond)

	var transitions []store.NotificationTransition
	engine.AddTransitionListener(func(s *store.Site, tr store.NotificationTransition, prev store.SiteStatus) {
		transitions = append(transitions, tr)
	})

	ctx := context.Background()

	// Check 1: Server healthy (initial probe: pending -> up)
	chk1, state1, err := engine.CheckSite(ctx, site)
	if err != nil || !chk1.IsUp {
		t.Fatalf("check 1 failed: %v", err)
	}
	if state1.CurrentStatus != store.SiteStatusUp {
		t.Errorf("expected status 'up', got %s", state1.CurrentStatus)
	}
	if state1.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 failures, got %d", state1.ConsecutiveFailures)
	}

	// Now trigger failure
	atomic.StoreInt32(&shouldFail, 1)

	// Check 2: First failure (consecutive_failures = 1 < threshold 2 -> remains 'up')
	chk2, state2, err := engine.CheckSite(ctx, site)
	if err != nil || chk2.IsUp {
		t.Fatalf("check 2 should be down, err: %v", err)
	}
	if state2.CurrentStatus != store.SiteStatusUp {
		t.Errorf("expected status to still be 'up' after 1 failure, got %s", state2.CurrentStatus)
	}
	if state2.ConsecutiveFailures != 1 {
		t.Errorf("expected consecutive failures 1, got %d", state2.ConsecutiveFailures)
	}

	// Check 3: Second failure (consecutive_failures = 2 == threshold 2 -> flips to 'down'!)
	chk3, state3, err := engine.CheckSite(ctx, site)
	if err != nil || chk3.IsUp {
		t.Fatalf("check 3 should be down, err: %v", err)
	}
	if state3.CurrentStatus != store.SiteStatusDown {
		t.Errorf("expected status to flip to 'down', got %s", state3.CurrentStatus)
	}
	if state3.ConsecutiveFailures != 2 {
		t.Errorf("expected consecutive failures 2, got %d", state3.ConsecutiveFailures)
	}

	// Check 4: Server recovers
	atomic.StoreInt32(&shouldFail, 0)
	chk4, state4, err := engine.CheckSite(ctx, site)
	if err != nil || !chk4.IsUp {
		t.Fatalf("check 4 should recover, err: %v", err)
	}
	if state4.CurrentStatus != store.SiteStatusUp {
		t.Errorf("expected status to recover to 'up', got %s", state4.CurrentStatus)
	}
	if state4.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures after recovery, got %d", state4.ConsecutiveFailures)
	}

	// Verify check_results table received 4 rows
	history, err := memStore.GetCheckResults(ctx, site.ID, 10, 0)
	if err != nil || len(history) != 4 {
		t.Fatalf("expected 4 recorded check results, got %d", len(history))
	}

	// Verify state transitions were notified:
	// 1: initial pending -> up (TransitionDownToUp)
	// 2: up -> down (TransitionUpToDown)
	// 3: down -> up (TransitionDownToUp)
	time.Sleep(50 * time.Millisecond) // wait for goroutines
	if len(transitions) != 3 {
		t.Errorf("expected 3 transition events, got %d: %v", len(transitions), transitions)
	}
}

func TestWorkerPool(t *testing.T) {
	wp := checker.NewWorkerPool(4, 20)

	var completed int32
	for i := 0; i < 10; i++ {
		ok := wp.Submit(func() {
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
		})
		if !ok {
			t.Fatal("failed to submit task to worker pool")
		}
	}

	// Graceful shutdown waits for tasks to finish
	stopped := wp.Stop(5 * time.Second)
	if !stopped {
		t.Fatal("worker pool did not stop cleanly")
	}

	if atomic.LoadInt32(&completed) != 10 {
		t.Errorf("expected all 10 tasks to complete, got %d", completed)
	}

	// Submitting after shutdown fails
	if wp.Submit(func() {}) {
		t.Error("submit should fail after pool is stopped")
	}
}

func TestSchedulerLifecycle(t *testing.T) {
	var checkCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&checkCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	memStore := store.NewMemoryStore()
	userID := uuid.New()
	site := &store.Site{
		ID:                uuid.New(),
		UserID:            userID,
		URL:               ts.URL,
		NormalizedURL:     "test-sched.com",
		CheckIntervalSec:  5,
		TimeoutSec:        2,
		FailureThreshold:  2,
		RetryCount:        0,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 299,
	}
	_, err := memStore.CreateSite(context.Background(), site, nil)
	if err != nil {
		t.Fatalf("failed to create site: %v", err)
	}

	engine := checker.NewEngine(memStore, ts.Client())
	pool := checker.NewWorkerPool(2, 10)
	sched := checker.NewScheduler(memStore, engine, pool, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}

	// Wait for immediate initial check
	time.Sleep(150 * time.Millisecond)
	if atomic.LoadInt32(&checkCount) == 0 {
		t.Error("expected at least 1 check to run after scheduler start")
	}

	// Trigger manual check
	if err := sched.TriggerCheck(ctx, site.ID); err != nil {
		t.Fatalf("failed to trigger check: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Graceful shutdown
	stopped := sched.Stop(5 * time.Second)
	if !stopped {
		t.Error("scheduler did not stop cleanly")
	}
}

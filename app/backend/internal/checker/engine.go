package checker

import (
	"context"
	"net/http"
	"sync"
	"time"

	"web-monitor/internal/store"
)

// TransitionListener is a callback function invoked when a site transitions between states.
type TransitionListener func(site *store.Site, transition store.NotificationTransition, previousStatus store.SiteStatus)

// Engine orchestrates the execution of site health checks, recording results and managing state transitions.
type Engine struct {
	store        store.Store
	httpClient   *http.Client
	mu           sync.RWMutex
	listeners    []TransitionListener
	retryBackoff time.Duration
}

// NewEngine creates a new check Engine instance.
func NewEngine(s store.Store, httpClient *http.Client) *Engine {
	if httpClient == nil {
		httpClient = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		}
	}

	return &Engine{
		store:        s,
		httpClient:   httpClient,
		retryBackoff: 200 * time.Millisecond,
	}
}

// SetRetryBackoff overrides the pause duration between retry attempts (useful in tests).
func (e *Engine) SetRetryBackoff(d time.Duration) {
	e.retryBackoff = d
}

// AddTransitionListener registers a listener to be notified on site status changes.
func (e *Engine) AddTransitionListener(listener TransitionListener) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners = append(e.listeners, listener)
}

// CheckSite runs a probe on the site, persists the check result, and evaluates state transitions.
func (e *Engine) CheckSite(ctx context.Context, site *store.Site) (*store.CheckResult, *store.SiteState, error) {
	now := time.Now().UTC()

	probeRes := ProbeWithRetry(ctx, e.httpClient, site, e.retryBackoff)

	checkResult := &store.CheckResult{
		SiteID:         site.ID,
		CheckedAt:      now,
		StatusCode:     probeRes.StatusCode,
		ResponseTimeMs: probeRes.ResponseTimeMs,
		IsUp:           probeRes.IsUp,
		ErrorType:      probeRes.ErrorType,
		ErrorMessage:   probeRes.ErrorMessage,
	}
	if probeRes.TimingBreakdown != nil {
		_ = checkResult.SetTimingBreakdown(probeRes.TimingBreakdown)
	}

	if err := e.store.RecordCheckResult(ctx, checkResult); err != nil {
		return nil, nil, err
	}

	state, err := e.store.GetSiteState(ctx, site.ID)
	if err != nil {
		state = &store.SiteState{
			SiteID:              site.ID,
			CurrentStatus:       store.SiteStatusPending,
			ConsecutiveFailures: 0,
		}
	}

	var transition *store.NotificationTransition
	previousStatus := state.CurrentStatus

	if probeRes.IsUp {
		state.ConsecutiveFailures = 0
		if state.CurrentStatus != store.SiteStatusUp {
			t := store.TransitionDownToUp
			transition = &t
			state.CurrentStatus = store.SiteStatusUp
			state.LastStatusChangeAt = &now
		}
	} else {
		state.ConsecutiveFailures++
		if state.ConsecutiveFailures >= site.FailureThreshold {
			if state.CurrentStatus != store.SiteStatusDown {
				t := store.TransitionUpToDown
				transition = &t
				state.CurrentStatus = store.SiteStatusDown
				state.LastStatusChangeAt = &now
			}
		}
	}
	state.LastCheckedAt = &now

	if err := e.store.UpdateSiteState(ctx, state); err != nil {
		return checkResult, nil, err
	}

	if transition != nil {
		e.notifyListeners(site, *transition, previousStatus)
	}

	return checkResult, state, nil
}

func (e *Engine) notifyListeners(site *store.Site, transition store.NotificationTransition, previousStatus store.SiteStatus) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, listener := range e.listeners {
		go listener(site, transition, previousStatus)
	}
}

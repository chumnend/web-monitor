package sites

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/store"
)

var (
	ErrInvalidCheckInterval    = errors.New("check interval must be between 10 and 86400 seconds")
	ErrInvalidTimeout          = errors.New("timeout must be between 1 and 60 seconds")
	ErrInvalidFailureThreshold = errors.New("failure threshold must be between 1 and 10")
	ErrInvalidRetryCount       = errors.New("retry count must be between 0 and 5")
	ErrInvalidExpectedStatus   = errors.New("expected status range must be between 100 and 599, with min <= max")
)

// CreateSiteInput holds the parameters for creating a new site to monitor.
type CreateSiteInput struct {
	URL               string   `json:"url"`
	Label             *string  `json:"label,omitempty"`
	CheckIntervalSec  int      `json:"check_interval_sec,omitempty"`
	TimeoutSec        int      `json:"timeout_sec,omitempty"`
	FailureThreshold  int      `json:"failure_threshold,omitempty"`
	RetryCount        *int     `json:"retry_count,omitempty"`
	ExpectedStatusMin int      `json:"expected_status_min,omitempty"`
	ExpectedStatusMax int      `json:"expected_status_max,omitempty"`
	Tags              []string `json:"tags,omitempty"`
}

// Service provides site monitoring management operations.
type Service struct {
	store store.Store
	cache *Cache
}

// NewService creates a new sites Service with optional cache TTL.
func NewService(s store.Store, cacheTTL ...time.Duration) *Service {
	ttl := 15 * time.Second
	if len(cacheTTL) > 0 && cacheTTL[0] > 0 {
		ttl = cacheTTL[0]
	}
	return &Service{
		store: s,
		cache: NewCache(ttl),
	}
}

// Close releases resources held by the service (such as cache cleanup tickers).
func (s *Service) Close() {
	if s.cache != nil {
		s.cache.Close()
	}
}

// CreateSite validates parameters, sets defaults, and persists a new site.
func (s *Service) CreateSite(ctx context.Context, userID uuid.UUID, input CreateSiteInput) (*store.SiteWithTags, error) {
	if userID == uuid.Nil {
		return nil, errors.New("unauthorized: missing user id")
	}

	displayURL, normalizedURL, err := ValidateAndNormalizeURL(input.URL)
	if err != nil {
		return nil, err
	}

	checkInterval := 60
	if input.CheckIntervalSec > 0 {
		if input.CheckIntervalSec < 10 || input.CheckIntervalSec > 86400 {
			return nil, ErrInvalidCheckInterval
		}
		checkInterval = input.CheckIntervalSec
	}

	timeout := 10
	if input.TimeoutSec > 0 {
		if input.TimeoutSec < 1 || input.TimeoutSec > 60 {
			return nil, ErrInvalidTimeout
		}
		timeout = input.TimeoutSec
	}

	threshold := 2
	if input.FailureThreshold > 0 {
		if input.FailureThreshold < 1 || input.FailureThreshold > 10 {
			return nil, ErrInvalidFailureThreshold
		}
		threshold = input.FailureThreshold
	}

	retryCount := 1
	if input.RetryCount != nil {
		if *input.RetryCount < 0 || *input.RetryCount > 5 {
			return nil, ErrInvalidRetryCount
		}
		retryCount = *input.RetryCount
	}

	minStatus := 200
	if input.ExpectedStatusMin > 0 {
		minStatus = input.ExpectedStatusMin
	}

	maxStatus := 399
	if input.ExpectedStatusMax > 0 {
		maxStatus = input.ExpectedStatusMax
	}

	if minStatus < 100 || minStatus > 599 || maxStatus < 100 || maxStatus > 599 || minStatus > maxStatus {
		return nil, ErrInvalidExpectedStatus
	}

	var label *string
	if input.Label != nil {
		trimmed := strings.TrimSpace(*input.Label)
		if trimmed != "" {
			label = &trimmed
		}
	}

	site := &store.Site{
		UserID:            userID,
		URL:               displayURL,
		NormalizedURL:     normalizedURL,
		Label:             label,
		CheckIntervalSec:  checkInterval,
		TimeoutSec:        timeout,
		FailureThreshold:  threshold,
		RetryCount:        retryCount,
		ExpectedStatusMin: minStatus,
		ExpectedStatusMax: maxStatus,
	}

	created, err := s.store.CreateSite(ctx, site, input.Tags)
	if err == nil {
		s.cache.InvalidateUser(userID)
	}
	return created, err
}

// GetSite retrieves an active site for the specified user.
func (s *Service) GetSite(ctx context.Context, userID, siteID uuid.UUID) (*store.SiteWithTags, error) {
	if userID == uuid.Nil || siteID == uuid.Nil {
		return nil, store.ErrSiteNotFound
	}
	return s.store.GetSiteByID(ctx, userID, siteID)
}

// ListSites lists all active sites for the specified user, optionally filtered by tag.
func (s *Service) ListSites(ctx context.Context, userID uuid.UUID, tagFilter string) ([]*store.SiteWithTags, error) {
	if userID == uuid.Nil {
		return nil, errors.New("unauthorized: missing user id")
	}
	return s.store.ListSites(ctx, userID, tagFilter)
}

// UpdateSite validates update parameters and updates an existing site.
func (s *Service) UpdateSite(ctx context.Context, userID, siteID uuid.UUID, input store.UpdateSiteInput) (*store.SiteWithTags, error) {
	if userID == uuid.Nil || siteID == uuid.Nil {
		return nil, store.ErrSiteNotFound
	}

	if input.CheckIntervalSec != nil {
		if *input.CheckIntervalSec < 10 || *input.CheckIntervalSec > 86400 {
			return nil, ErrInvalidCheckInterval
		}
	}

	if input.TimeoutSec != nil {
		if *input.TimeoutSec < 1 || *input.TimeoutSec > 60 {
			return nil, ErrInvalidTimeout
		}
	}

	if input.FailureThreshold != nil {
		if *input.FailureThreshold < 1 || *input.FailureThreshold > 10 {
			return nil, ErrInvalidFailureThreshold
		}
	}

	if input.RetryCount != nil {
		if *input.RetryCount < 0 || *input.RetryCount > 5 {
			return nil, ErrInvalidRetryCount
		}
	}

	if input.ExpectedStatusMin != nil || input.ExpectedStatusMax != nil {
		min := 200
		max := 399
		current, err := s.store.GetSiteByID(ctx, userID, siteID)
		if err != nil {
			return nil, err
		}
		min = current.ExpectedStatusMin
		max = current.ExpectedStatusMax

		if input.ExpectedStatusMin != nil {
			min = *input.ExpectedStatusMin
		}
		if input.ExpectedStatusMax != nil {
			max = *input.ExpectedStatusMax
		}

		if min < 100 || min > 599 || max < 100 || max > 599 || min > max {
			return nil, ErrInvalidExpectedStatus
		}
	}

	if input.Label != nil {
		trimmed := strings.TrimSpace(*input.Label)
		input.Label = &trimmed
	}

	updated, err := s.store.UpdateSite(ctx, userID, siteID, input)
	if err == nil {
		s.cache.InvalidateUser(userID)
		s.cache.InvalidateSite(siteID)
	}
	return updated, err
}

// DeleteSite soft-deletes a site for the specified user.
func (s *Service) DeleteSite(ctx context.Context, userID, siteID uuid.UUID) error {
	if userID == uuid.Nil || siteID == uuid.Nil {
		return store.ErrSiteNotFound
	}
	err := s.store.DeleteSite(ctx, userID, siteID)
	if err == nil {
		s.cache.InvalidateUser(userID)
		s.cache.InvalidateSite(siteID)
	}
	return err
}

// GetSiteHistory retrieves paginated check results for a site after verifying ownership.
func (s *Service) GetSiteHistory(ctx context.Context, userID, siteID uuid.UUID, limit, offset int) ([]*store.CheckResult, int, error) {
	if userID == uuid.Nil || siteID == uuid.Nil {
		return nil, 0, store.ErrSiteNotFound
	}

	// Verify site ownership
	if _, err := s.store.GetSiteByID(ctx, userID, siteID); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	results, err := s.store.GetCheckResults(ctx, siteID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.store.CountCheckResults(ctx, siteID)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// GetSiteUptime computes or returns cached uptime statistics for a site after verifying ownership.
func (s *Service) GetSiteUptime(ctx context.Context, userID, siteID uuid.UUID) (*store.SiteUptimeStats, error) {
	if userID == uuid.Nil || siteID == uuid.Nil {
		return nil, store.ErrSiteNotFound
	}

	// Verify site ownership
	if _, err := s.store.GetSiteByID(ctx, userID, siteID); err != nil {
		return nil, err
	}

	cacheKey := "uptime:" + siteID.String()
	if cached, ok := s.cache.Get(cacheKey); ok {
		if stats, ok := cached.(*store.SiteUptimeStats); ok {
			return stats, nil
		}
	}

	stats, err := s.store.GetSiteUptimeStats(ctx, siteID)
	if err != nil {
		return nil, err
	}

	s.cache.Set(cacheKey, stats)
	return stats, nil
}

// GetDashboardSummary retrieves or computes cached dashboard summary counts for a user.
func (s *Service) GetDashboardSummary(ctx context.Context, userID uuid.UUID) (*store.DashboardSummary, error) {
	if userID == uuid.Nil {
		return nil, errors.New("unauthorized: missing user id")
	}

	cacheKey := "summary:" + userID.String()
	if cached, ok := s.cache.Get(cacheKey); ok {
		if summary, ok := cached.(*store.DashboardSummary); ok {
			return summary, nil
		}
	}

	summary, err := s.store.GetDashboardSummary(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.cache.Set(cacheKey, summary)
	return summary, nil
}

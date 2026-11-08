package sites

import (
	"context"
	"errors"
	"strings"

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
}

// NewService creates a new sites Service.
func NewService(s store.Store) *Service {
	return &Service{store: s}
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

	return s.store.CreateSite(ctx, site, input.Tags)
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
		// Fetch current site to validate min <= max if only one is updated
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

	return s.store.UpdateSite(ctx, userID, siteID, input)
}

// DeleteSite soft-deletes a site for the specified user.
func (s *Service) DeleteSite(ctx context.Context, userID, siteID uuid.UUID) error {
	if userID == uuid.Nil || siteID == uuid.Nil {
		return store.ErrSiteNotFound
	}
	return s.store.DeleteSite(ctx, userID, siteID)
}

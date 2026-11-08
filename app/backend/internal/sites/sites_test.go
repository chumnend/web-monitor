package sites_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"web-monitor/internal/sites"
	"web-monitor/internal/store"
)

func TestValidateAndNormalizeURL(t *testing.T) {
	tests := []struct {
		name              string
		raw               string
		expectedDisplay   string
		expectedNormalize string
		expectErr         bool
	}{
		{
			name:              "standard https URL",
			raw:               "https://example.com/api",
			expectedDisplay:   "https://example.com/api",
			expectedNormalize: "example.com/api",
			expectErr:         false,
		},
		{
			name:              "missing scheme defaults to https",
			raw:               "example.com/api",
			expectedDisplay:   "https://example.com/api",
			expectedNormalize: "example.com/api",
			expectErr:         false,
		},
		{
			name:              "strips default port 443",
			raw:               "https://EXAMPLE.COM:443/api",
			expectedDisplay:   "https://example.com/api",
			expectedNormalize: "example.com/api",
			expectErr:         false,
		},
		{
			name:              "strips default port 80",
			raw:               "http://example.com:80/",
			expectedDisplay:   "http://example.com",
			expectedNormalize: "example.com",
			expectErr:         false,
		},
		{
			name:              "preserves non-default port",
			raw:               "http://example.com:8080/test",
			expectedDisplay:   "http://example.com:8080/test",
			expectedNormalize: "example.com:8080/test",
			expectErr:         false,
		},
		{
			name:              "sorts query parameters and strips fragment",
			raw:               "https://example.com/api?b=2&a=1#section",
			expectedDisplay:   "https://example.com/api?b=2&a=1",
			expectedNormalize: "example.com/api?a=1&b=2",
			expectErr:         false,
		},
		{
			name:      "empty URL",
			raw:       "",
			expectErr: true,
		},
		{
			name:      "unsupported scheme",
			raw:       "ftp://example.com/file",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			display, normalized, err := sites.ValidateAndNormalizeURL(tc.raw)
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if display != tc.expectedDisplay {
					t.Errorf("display expected %q, got %q", tc.expectedDisplay, display)
				}
				if normalized != tc.expectedNormalize {
					t.Errorf("normalized expected %q, got %q", tc.expectedNormalize, normalized)
				}
			}
		})
	}
}

func TestServiceValidation(t *testing.T) {
	memStore := store.NewMemoryStore()
	svc := sites.NewService(memStore)
	ctx := context.Background()
	userID := uuid.New()

	// 1. Invalid Check Interval
	_, err := svc.CreateSite(ctx, userID, sites.CreateSiteInput{
		URL:              "example.com",
		CheckIntervalSec: 5, // minimum 10
	})
	if err != sites.ErrInvalidCheckInterval {
		t.Errorf("expected ErrInvalidCheckInterval, got %v", err)
	}

	// 2. Invalid Timeout
	_, err = svc.CreateSite(ctx, userID, sites.CreateSiteInput{
		URL:        "example.com",
		TimeoutSec: 100, // max 60
	})
	if err != sites.ErrInvalidTimeout {
		t.Errorf("expected ErrInvalidTimeout, got %v", err)
	}

	// 3. Invalid Failure Threshold
	_, err = svc.CreateSite(ctx, userID, sites.CreateSiteInput{
		URL:              "example.com",
		FailureThreshold: 20, // max 10
	})
	if err != sites.ErrInvalidFailureThreshold {
		t.Errorf("expected ErrInvalidFailureThreshold, got %v", err)
	}

	// 4. Invalid Retry Count
	badRetry := 10
	_, err = svc.CreateSite(ctx, userID, sites.CreateSiteInput{
		URL:        "example.com",
		RetryCount: &badRetry, // max 5
	})
	if err != sites.ErrInvalidRetryCount {
		t.Errorf("expected ErrInvalidRetryCount, got %v", err)
	}

	// 5. Invalid Expected Status Range (min > max)
	_, err = svc.CreateSite(ctx, userID, sites.CreateSiteInput{
		URL:               "example.com",
		ExpectedStatusMin: 400,
		ExpectedStatusMax: 200,
	})
	if err != sites.ErrInvalidExpectedStatus {
		t.Errorf("expected ErrInvalidExpectedStatus, got %v", err)
	}
}

func TestServiceCRUD(t *testing.T) {
	memStore := store.NewMemoryStore()
	svc := sites.NewService(memStore)
	ctx := context.Background()
	userID := uuid.New()

	// 1. Create with defaults
	created, err := svc.CreateSite(ctx, userID, sites.CreateSiteInput{
		URL:  "https://example.com",
		Tags: []string{"prod", "web"},
	})
	if err != nil {
		t.Fatalf("failed to create site: %v", err)
	}

	if created.CheckIntervalSec != 60 {
		t.Errorf("expected default interval 60, got %d", created.CheckIntervalSec)
	}
	if created.TimeoutSec != 10 {
		t.Errorf("expected default timeout 10, got %d", created.TimeoutSec)
	}
	if created.FailureThreshold != 2 {
		t.Errorf("expected default threshold 2, got %d", created.FailureThreshold)
	}
	if created.RetryCount != 1 {
		t.Errorf("expected default retry count 1, got %d", created.RetryCount)
	}
	if created.ExpectedStatusMin != 200 || created.ExpectedStatusMax != 399 {
		t.Errorf("expected status 200-399, got %d-%d", created.ExpectedStatusMin, created.ExpectedStatusMax)
	}
	if created.CurrentStatus != store.SiteStatusPending {
		t.Errorf("expected pending status, got %s", created.CurrentStatus)
	}

	// 2. Get Site
	fetched, err := svc.GetSite(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("failed to get site: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("ID mismatch: %v vs %v", fetched.ID, created.ID)
	}

	// 3. List Sites
	list, err := svc.ListSites(ctx, userID, "")
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 site in list, got %d (err: %v)", len(list), err)
	}

	// 4. Update Site
	newInterval := 120
	updated, err := svc.UpdateSite(ctx, userID, created.ID, store.UpdateSiteInput{
		CheckIntervalSec: &newInterval,
		Tags:             &[]string{"staging"},
	})
	if err != nil {
		t.Fatalf("failed to update site: %v", err)
	}
	if updated.CheckIntervalSec != 120 {
		t.Errorf("expected interval 120, got %d", updated.CheckIntervalSec)
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "staging" {
		t.Errorf("expected updated tags ['staging'], got %v", updated.Tags)
	}

	// 5. Delete Site
	if err := svc.DeleteSite(ctx, userID, created.ID); err != nil {
		t.Fatalf("failed to delete site: %v", err)
	}
	_, err = svc.GetSite(ctx, userID, created.ID)
	if err != store.ErrSiteNotFound {
		t.Errorf("expected ErrSiteNotFound after delete, got %v", err)
	}
}

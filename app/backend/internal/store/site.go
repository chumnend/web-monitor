package store

import (
	"time"

	"github.com/google/uuid"
)

// Site represents a website configured for uptime and health monitoring (from 000002_create_sites_table).
type Site struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	UserID            uuid.UUID  `json:"user_id" db:"user_id"`
	URL               string     `json:"url" db:"url"`
	NormalizedURL     string     `json:"normalized_url" db:"normalized_url"`
	Label             *string    `json:"label,omitempty" db:"label"`
	CheckIntervalSec  int        `json:"check_interval_sec" db:"check_interval_sec"`
	TimeoutSec        int        `json:"timeout_sec" db:"timeout_sec"`
	FailureThreshold  int        `json:"failure_threshold" db:"failure_threshold"`
	RetryCount        int        `json:"retry_count" db:"retry_count"`
	ExpectedStatusMin int        `json:"expected_status_min" db:"expected_status_min"`
	ExpectedStatusMax int        `json:"expected_status_max" db:"expected_status_max"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// Tag represents a user-defined tag for organizing monitored sites (from 000003_create_tags_table).
type Tag struct {
	ID     uuid.UUID `json:"id" db:"id"`
	UserID uuid.UUID `json:"user_id" db:"user_id"`
	Name   string    `json:"name" db:"name"`
}

// SiteTag represents the many-to-many relationship between sites and tags (from 000004_create_site_tags_table).
type SiteTag struct {
	SiteID uuid.UUID `json:"site_id" db:"site_id"`
	TagID  uuid.UUID `json:"tag_id" db:"tag_id"`
}

// SiteStatus represents the status of a monitored site.
type SiteStatus string

const (
	SiteStatusUp      SiteStatus = "up"
	SiteStatusDown    SiteStatus = "down"
	SiteStatusPending SiteStatus = "pending"
)

// SiteState represents the live status and roll-up metrics for a site (from 000006_create_site_state_table).
type SiteState struct {
	SiteID              uuid.UUID  `json:"site_id" db:"site_id"`
	CurrentStatus       SiteStatus `json:"current_status" db:"current_status"`
	ConsecutiveFailures int        `json:"consecutive_failures" db:"consecutive_failures"`
	LastCheckedAt       *time.Time `json:"last_checked_at,omitempty" db:"last_checked_at"`
	LastStatusChangeAt  *time.Time `json:"last_status_change_at,omitempty" db:"last_status_change_at"`
	UptimePct24h        *float64   `json:"uptime_pct_24h,omitempty" db:"uptime_pct_24h"`
	UptimePct30d        *float64   `json:"uptime_pct_30d,omitempty" db:"uptime_pct_30d"`
}

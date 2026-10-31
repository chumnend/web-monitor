package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// User represents a user account in the system (from 000001_create_users_table).
type User struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	Email           string     `json:"email" db:"email"`
	PasswordHash    string     `json:"password_hash" db:"password_hash"`
	Timezone        string     `json:"timezone" db:"timezone"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty" db:"email_verified_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
}

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

// TimingBreakdown provides detailed network timing metrics (in milliseconds) for a check probe.
type TimingBreakdown struct {
	DNSLookupMs    *int `json:"dns_lookup_ms,omitempty"`
	TCPConnectMs   *int `json:"tcp_connect_ms,omitempty"`
	TLSHandshakeMs *int `json:"tls_handshake_ms,omitempty"`
	FirstByteMs    *int `json:"first_byte_ms,omitempty"`
	TotalMs        *int `json:"total_ms,omitempty"`
}

// Value implements the database/sql/driver.Valuer interface.
func (t TimingBreakdown) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// Scan implements the database/sql.Scanner interface.
func (t *TimingBreakdown) Scan(value any) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to scan TimingBreakdown: expected []byte or string, got %T", value)
	}
	return json.Unmarshal(bytes, t)
}

// Common error types for check results.
const (
	ErrorTypeTimeout           = "timeout"
	ErrorTypeDNS               = "dns"
	ErrorTypeTLS               = "tls"
	ErrorTypeConnectionRefused = "connection_refused"
)

// CheckResult represents the outcome of an individual health check for a site (from 000005_create_check_results_table).
type CheckResult struct {
	ID              int64           `json:"id" db:"id"`
	SiteID          uuid.UUID       `json:"site_id" db:"site_id"`
	CheckedAt       time.Time       `json:"checked_at" db:"checked_at"`
	StatusCode      *int            `json:"status_code,omitempty" db:"status_code"`
	ResponseTimeMs  *int            `json:"response_time_ms,omitempty" db:"response_time_ms"`
	IsUp            bool            `json:"is_up" db:"is_up"`
	ErrorType       *string         `json:"error_type,omitempty" db:"error_type"`
	ErrorMessage    *string         `json:"error_message,omitempty" db:"error_message"`
	TimingBreakdown json.RawMessage `json:"timing_breakdown,omitempty" db:"timing_breakdown"`
}

// ParseTimingBreakdown unmarshals the raw JSON timing breakdown into a TimingBreakdown struct.
func (c *CheckResult) ParseTimingBreakdown() (*TimingBreakdown, error) {
	if len(c.TimingBreakdown) == 0 {
		return nil, nil
	}
	var tb TimingBreakdown
	if err := json.Unmarshal(c.TimingBreakdown, &tb); err != nil {
		return nil, err
	}
	return &tb, nil
}

// SetTimingBreakdown serializes a TimingBreakdown struct into the raw JSON timing breakdown field.
func (c *CheckResult) SetTimingBreakdown(tb *TimingBreakdown) error {
	if tb == nil {
		c.TimingBreakdown = nil
		return nil
	}
	data, err := json.Marshal(tb)
	if err != nil {
		return err
	}
	c.TimingBreakdown = data
	return nil
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

// ChannelType represents the notification delivery channel type.
type ChannelType string

const (
	ChannelTypeEmail   ChannelType = "email"
	ChannelTypeWebhook ChannelType = "webhook"
	ChannelTypePush    ChannelType = "push"
)

// NotificationChannel represents a configured delivery target for site alerts (from 000007_create_notification_channels_table).
type NotificationChannel struct {
	ID        uuid.UUID   `json:"id" db:"id"`
	UserID    uuid.UUID   `json:"user_id" db:"user_id"`
	SiteID    *uuid.UUID  `json:"site_id,omitempty" db:"site_id"`
	Type      ChannelType `json:"type" db:"type"`
	Target    string      `json:"target" db:"target"`
	IsEnabled bool        `json:"is_enabled" db:"is_enabled"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
}

// NotificationTransition represents the site state change triggering a notification.
type NotificationTransition string

const (
	TransitionUpToDown NotificationTransition = "up_to_down"
	TransitionDownToUp NotificationTransition = "down_to_up"
)

// Delivery status constants for notification events.
const (
	DeliveryStatusSent               = "sent"
	DeliveryStatusFailed             = "failed"
	DeliveryStatusSuppressedDebounce = "suppressed_debounce"
)

// NotificationEvent records a triggered notification event (from 000008_create_notification_events_table).
type NotificationEvent struct {
	ID             uuid.UUID              `json:"id" db:"id"`
	SiteID         uuid.UUID              `json:"site_id" db:"site_id"`
	ChannelID      uuid.UUID              `json:"channel_id" db:"channel_id"`
	Transition     NotificationTransition `json:"transition" db:"transition"`
	SentAt         time.Time              `json:"sent_at" db:"sent_at"`
	DeliveryStatus *string                `json:"delivery_status,omitempty" db:"delivery_status"`
}

// RefreshToken represents a stored refresh token in the system (from 000009_create_refresh_tokens_table).
type RefreshToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	TokenHash string     `json:"token_hash" db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}


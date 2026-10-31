package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Common error types for check results.
const (
	ErrorTypeTimeout           = "timeout"
	ErrorTypeDNS               = "dns"
	ErrorTypeTLS               = "tls"
	ErrorTypeConnectionRefused = "connection_refused"
)

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

package store

import (
	"time"

	"github.com/google/uuid"
)

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

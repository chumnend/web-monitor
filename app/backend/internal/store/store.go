package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrDuplicateEmail       = errors.New("email already exists")
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrSiteNotFound         = errors.New("site not found")
	ErrDuplicateSite        = errors.New("site with this URL already exists")
	ErrSiteStateNotFound    = errors.New("site state not found")
	ErrChannelNotFound      = errors.New("notification channel not found")
)

// Store defines the data access contract for the application.
type Store interface {
	Ping(ctx context.Context) error

	// Users
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)

	// Refresh Tokens
	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string, revokedAt time.Time) error
	RevokeUserRefreshTokens(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error

	// Sites & Tags
	CreateSite(ctx context.Context, site *Site, tags []string) (*SiteWithTags, error)
	GetSiteByID(ctx context.Context, userID, siteID uuid.UUID) (*SiteWithTags, error)
	ListSites(ctx context.Context, userID uuid.UUID, tagFilter string) ([]*SiteWithTags, error)
	GetAllActiveSites(ctx context.Context) ([]*Site, error)
	UpdateSite(ctx context.Context, userID, siteID uuid.UUID, input UpdateSiteInput) (*SiteWithTags, error)
	DeleteSite(ctx context.Context, userID, siteID uuid.UUID) error

	// Checks & Site State
	RecordCheckResult(ctx context.Context, result *CheckResult) error
	CountCheckResults(ctx context.Context, siteID uuid.UUID) (int, error)
	GetCheckResults(ctx context.Context, siteID uuid.UUID, limit, offset int) ([]*CheckResult, error)
	UpdateSiteState(ctx context.Context, state *SiteState) error
	GetSiteState(ctx context.Context, siteID uuid.UUID) (*SiteState, error)
	GetSiteUptimeStats(ctx context.Context, siteID uuid.UUID) (*SiteUptimeStats, error)
	GetDashboardSummary(ctx context.Context, userID uuid.UUID) (*DashboardSummary, error)

	// Notifications
	CreateNotificationChannel(ctx context.Context, ch *NotificationChannel) error
	GetNotificationChannelByID(ctx context.Context, userID, channelID uuid.UUID) (*NotificationChannel, error)
	ListNotificationChannels(ctx context.Context, userID uuid.UUID, siteID *uuid.UUID) ([]*NotificationChannel, error)
	UpdateNotificationChannel(ctx context.Context, userID, channelID uuid.UUID, isEnabled *bool, target *string) (*NotificationChannel, error)
	DeleteNotificationChannel(ctx context.Context, userID, channelID uuid.UUID) error
	GetChannelsForSiteTransition(ctx context.Context, site *Site) ([]*NotificationChannel, error)
	RecordNotificationEvent(ctx context.Context, event *NotificationEvent) error
	GetLastNotificationEvent(ctx context.Context, siteID, channelID uuid.UUID) (*NotificationEvent, error)
	ListNotificationEvents(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*NotificationEvent, error)
}

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
	GetCheckResults(ctx context.Context, siteID uuid.UUID, limit, offset int) ([]*CheckResult, error)
	UpdateSiteState(ctx context.Context, state *SiteState) error
	GetSiteState(ctx context.Context, siteID uuid.UUID) (*SiteState, error)
}

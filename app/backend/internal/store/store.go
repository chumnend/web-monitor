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
)

// Store defines the data access contract for the application.
type Store interface {
	Ping(ctx context.Context) error

	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)

	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string, revokedAt time.Time) error
	RevokeUserRefreshTokens(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error
}

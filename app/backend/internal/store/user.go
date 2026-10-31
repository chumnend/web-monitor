package store

import (
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

// RefreshToken represents a stored refresh token in the system (from 000009_create_refresh_tokens_table).
type RefreshToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	TokenHash string     `json:"token_hash" db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

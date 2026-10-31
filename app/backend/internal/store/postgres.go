package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PostgresStore implements Store using a PostgreSQL database.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates a new PostgresStore instance.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// DB returns the underlying *sql.DB connection pool.
func (s *PostgresStore) DB() *sql.DB {
	return s.db
}

// Ping verifies that the database connection is alive.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// CreateUser inserts a new user record into the users table.
func (s *PostgresStore) CreateUser(ctx context.Context, user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	if user.Timezone == "" {
		user.Timezone = "UTC"
	}
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))

	query := `
		INSERT INTO users (id, email, password_hash, timezone, email_verified_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Timezone,
		user.EmailVerifiedAt,
		user.CreatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrDuplicateEmail
		}
		return err
	}

	return nil
}

// GetUserByEmail retrieves a user by email address (case-insensitive).
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	query := `
		SELECT id, email, password_hash, timezone, email_verified_at, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`

	var user User
	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Timezone,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByID retrieves a user by their UUID.
func (s *PostgresStore) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, email, password_hash, timezone, email_verified_at, created_at
		FROM users
		WHERE id = $1
	`

	var user User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Timezone,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// CreateRefreshToken inserts a new refresh token into the refresh_tokens table.
func (s *PostgresStore) CreateRefreshToken(ctx context.Context, token *RefreshToken) error {
	if token == nil {
		return errors.New("refresh token cannot be nil")
	}
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := s.db.ExecContext(
		ctx,
		query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.RevokedAt,
		token.CreatedAt,
	)
	return err
}

// GetRefreshToken retrieves a refresh token by its hash.
func (s *PostgresStore) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var token RefreshToken
	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, err
	}
	return &token, nil
}

// RevokeRefreshToken marks a specific refresh token as revoked.
func (s *PostgresStore) RevokeRefreshToken(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1
		WHERE token_hash = $2 AND revoked_at IS NULL
	`
	res, err := s.db.ExecContext(ctx, query, revokedAt, tokenHash)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		var count int
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE token_hash = $1`, tokenHash).Scan(&count)
		if count == 0 {
			return ErrRefreshTokenNotFound
		}
	}
	return nil
}

// RevokeUserRefreshTokens revokes all active refresh tokens belonging to a user.
func (s *PostgresStore) RevokeUserRefreshTokens(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = $1
		WHERE user_id = $2 AND revoked_at IS NULL
	`
	_, err := s.db.ExecContext(ctx, query, revokedAt, userID)
	return err
}


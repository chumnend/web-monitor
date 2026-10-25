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

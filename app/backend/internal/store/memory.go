package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore provides an in-memory implementation of the Store interface for testing.
type MemoryStore struct {
	mu            sync.RWMutex
	users         map[uuid.UUID]*User
	refreshTokens map[string]*RefreshToken
}

// NewMemoryStore creates a new in-memory store instance.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:         make(map[uuid.UUID]*User),
		refreshTokens: make(map[string]*RefreshToken),
	}
}

// CreateUser creates a new user in memory.
func (m *MemoryStore) CreateUser(_ context.Context, user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedEmail := strings.ToLower(strings.TrimSpace(user.Email))

	// Check unique email constraint
	for _, u := range m.users {
		if strings.ToLower(u.Email) == normalizedEmail {
			return ErrDuplicateEmail
		}
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
	user.Email = normalizedEmail

	// Clone user to avoid mutations
	clone := *user
	m.users[user.ID] = &clone

	return nil
}

// GetUserByEmail finds a user by email in memory.
func (m *MemoryStore) GetUserByEmail(_ context.Context, email string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for _, u := range m.users {
		if strings.ToLower(u.Email) == normalizedEmail {
			clone := *u
			return &clone, nil
		}
	}

	return nil, ErrUserNotFound
}

// GetUserByID finds a user by ID in memory.
func (m *MemoryStore) GetUserByID(_ context.Context, id uuid.UUID) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}

	clone := *u
	return &clone, nil
}

// CreateRefreshToken stores a new refresh token in memory.
func (m *MemoryStore) CreateRefreshToken(_ context.Context, token *RefreshToken) error {
	if token == nil {
		return errors.New("refresh token cannot be nil")
	}
	if token.TokenHash == "" {
		return errors.New("token hash cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}

	clone := *token
	m.refreshTokens[token.TokenHash] = &clone

	return nil
}

// GetRefreshToken retrieves a refresh token by its hash.
func (m *MemoryStore) GetRefreshToken(_ context.Context, tokenHash string) (*RefreshToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, ok := m.refreshTokens[tokenHash]
	if !ok {
		return nil, ErrRefreshTokenNotFound
	}

	clone := *token
	return &clone, nil
}

// RevokeRefreshToken marks a specific refresh token as revoked.
func (m *MemoryStore) RevokeRefreshToken(_ context.Context, tokenHash string, revokedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.refreshTokens[tokenHash]
	if !ok {
		return ErrRefreshTokenNotFound
	}

	token.RevokedAt = &revokedAt
	return nil
}

// RevokeUserRefreshTokens revokes all active refresh tokens belonging to a user.
func (m *MemoryStore) RevokeUserRefreshTokens(_ context.Context, userID uuid.UUID, revokedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, token := range m.refreshTokens {
		if token.UserID == userID && token.RevokedAt == nil {
			token.RevokedAt = &revokedAt
		}
	}

	return nil
}

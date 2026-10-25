package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/store"
)

var (
	ErrUserAlreadyExists = errors.New("user with this email already exists")
)

// RegisterInput contains the input parameters for registering a new user.
type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Timezone string `json:"timezone,omitempty"`
}

// Service provides authentication and user management operations.
type Service struct {
	store store.Store
}

// NewService creates a new authentication service.
func NewService(s store.Store) *Service {
	return &Service{store: s}
}

// Register registers a new user with a hashed password using bcrypt.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*store.User, error) {
	email, err := ValidateAndNormalizeEmail(input.Email)
	if err != nil {
		return nil, err
	}

	hash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	tz := NormalizeTimezone(input.Timezone)

	now := time.Now().UTC()
	user := &store.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Timezone:     tz,
		CreatedAt:    now,
	}

	err = s.store.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateEmail) {
			return nil, ErrUserAlreadyExists
		}
		return nil, err
	}

	return user, nil
}

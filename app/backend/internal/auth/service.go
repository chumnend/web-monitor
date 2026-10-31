package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/store"
)

var (
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// RegisterInput contains the input parameters for registering a new user.
type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Timezone string `json:"timezone,omitempty"`
}

// LoginInput contains the input credentials for user login.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResult represents the outcome of a successful login.
type LoginResult struct {
	Token     string      `json:"token"`
	User      *store.User `json:"user"`
	ExpiresIn int64       `json:"expires_in"` // in seconds
}

// Service provides authentication and user management operations.
type Service struct {
	store        store.Store
	tokenManager *TokenManager
}

// NewService creates a new authentication service.
func NewService(s store.Store, opts ...func(*Service)) *Service {
	svc := &Service{
		store:        s,
		tokenManager: NewTokenManager("", 24*time.Hour),
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// WithTokenManager sets a custom TokenManager for the Service.
func WithTokenManager(tm *TokenManager) func(*Service) {
	return func(s *Service) {
		s.tokenManager = tm
	}
}

// TokenManager returns the service's TokenManager.
func (s *Service) TokenManager() *TokenManager {
	return s.tokenManager
}

// Middleware returns an HTTP Middleware using the service's TokenManager and Store.
func (s *Service) Middleware() *Middleware {
	return NewMiddleware(s.tokenManager, s.store)
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

// Login verifies user credentials and issues a JWT token.
func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || input.Password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := ComparePassword(user.PasswordHash, input.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokenManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Token:     token,
		User:      user,
		ExpiresIn: int64(s.tokenManager.Duration().Seconds()),
	}, nil
}

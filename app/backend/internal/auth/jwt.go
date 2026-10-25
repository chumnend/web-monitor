package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

// Claims represents the JWT claims payload.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// TokenManager manages signing and validating JWT tokens.
type TokenManager struct {
	secretKey []byte
	duration  time.Duration
}

// NewTokenManager creates a new TokenManager instance.
func NewTokenManager(secretKey string, duration time.Duration) *TokenManager {
	if secretKey == "" {
		secretKey = "default-development-secret-key-change-in-prod"
	}
	if duration == 0 {
		duration = 24 * time.Hour
	}
	return &TokenManager{
		secretKey: []byte(secretKey),
		duration:  duration,
	}
}

// Duration returns the configured token validity duration.
func (m *TokenManager) Duration() time.Duration {
	return m.duration
}

// GenerateToken generates and signs a new JWT token for a given user.
func (m *TokenManager) GenerateToken(userID uuid.UUID, email string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// ValidateToken parses and validates a JWT token string.
func (m *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secretKey, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

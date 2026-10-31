package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken        = errors.New("invalid or expired token")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
)

// Claims represents the JWT claims payload.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

// TokenManager manages signing and validating JWT tokens.
type TokenManager struct {
	secretKey       []byte
	duration        time.Duration
	refreshDuration time.Duration
}

// NewTokenManager creates a new TokenManager instance.
func NewTokenManager(secretKey string, duration time.Duration, refreshDuration ...time.Duration) *TokenManager {
	if secretKey == "" {
		secretKey = "default-development-secret-key-change-in-prod"
	}
	if duration == 0 {
		duration = 24 * time.Hour
	}

	refDuration := 7 * 24 * time.Hour
	if len(refreshDuration) > 0 && refreshDuration[0] != 0 {
		refDuration = refreshDuration[0]
	}

	return &TokenManager{
		secretKey:       []byte(secretKey),
		duration:        duration,
		refreshDuration: refDuration,
	}
}

// Duration returns the configured token validity duration.
func (m *TokenManager) Duration() time.Duration {
	return m.duration
}

// RefreshTokenDuration returns the configured refresh token validity duration.
func (m *TokenManager) RefreshTokenDuration() time.Duration {
	return m.refreshDuration
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

// GenerateRefreshToken generates a cryptographically secure random token.
func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashRefreshToken returns the SHA-256 hex digest of a raw refresh token.
func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

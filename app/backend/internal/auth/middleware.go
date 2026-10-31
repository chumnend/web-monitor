package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"web-monitor/internal/store"
)

var (
	ErrMissingAuthHeader = errors.New("missing authorization header")
	ErrInvalidAuthHeader = errors.New("invalid authorization header format")
)

// ExtractBearerToken extracts the bearer token from the Authorization header of an HTTP request.
func ExtractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingAuthHeader
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", ErrInvalidAuthHeader
	}

	return strings.TrimSpace(parts[1]), nil
}

// Middleware provides HTTP middleware for JWT authentication.
type Middleware struct {
	tokenManager *TokenManager
	store        store.Store
}

// NewMiddleware creates a new auth Middleware.
func NewMiddleware(tm *TokenManager, s store.Store) *Middleware {
	return &Middleware{
		tokenManager: tm,
		store:        s,
	}
}

// Handler returns an HTTP middleware handler that validates the JWT token
// and attaches both the user claims and user model to the request context.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr, err := ExtractBearerToken(r)
		if err != nil {
			m.writeError(w, http.StatusUnauthorized, "Missing or invalid authorization header")
			return
		}

		claims, err := m.tokenManager.ValidateToken(tokenStr)
		if err != nil {
			m.writeError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		ctx := WithClaims(r.Context(), claims)

		if m.store != nil {
			user, err := m.store.GetUserByID(ctx, claims.UserID)
			if err != nil {
				m.writeError(w, http.StatusUnauthorized, "User not found")
				return
			}
			ctx = WithUser(ctx, user)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WrapFunc provides a convenient wrapper for http.HandlerFunc.
func (m *Middleware) WrapFunc(fn http.HandlerFunc) http.HandlerFunc {
	return m.Handler(fn).ServeHTTP
}

func (m *Middleware) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

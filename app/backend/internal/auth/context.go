package auth

import (
	"context"

	"github.com/google/uuid"
	"web-monitor/internal/store"
)

type contextKey struct {
	name string
}

var (
	claimsContextKey = &contextKey{name: "user_claims"}
	userContextKey   = &contextKey{name: "user"}
)

// WithClaims returns a new context with the given JWT claims.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

// ClaimsFromContext retrieves the JWT claims from the context if present.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*Claims)
	return claims, ok && claims != nil
}

// WithUser returns a new context with the given authenticated user.
func WithUser(ctx context.Context, user *store.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user from the context if present.
func UserFromContext(ctx context.Context) (*store.User, bool) {
	user, ok := ctx.Value(userContextKey).(*store.User)
	return user, ok && user != nil
}

// UserIDFromContext retrieves the authenticated user's ID from context.
// It checks the user object first, falling back to claims.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	if user, ok := UserFromContext(ctx); ok && user.ID != uuid.Nil {
		return user.ID, true
	}
	if claims, ok := ClaimsFromContext(ctx); ok && claims.UserID != uuid.Nil {
		return claims.UserID, true
	}
	return uuid.Nil, false
}

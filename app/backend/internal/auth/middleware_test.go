package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/auth"
	"web-monitor/internal/store"
)

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Initial empty context
	if _, ok := auth.ClaimsFromContext(ctx); ok {
		t.Error("expected false for empty claims")
	}
	if _, ok := auth.UserFromContext(ctx); ok {
		t.Error("expected false for empty user")
	}
	if _, ok := auth.UserIDFromContext(ctx); ok {
		t.Error("expected false for empty user ID")
	}

	userID := uuid.New()
	claims := &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
	}

	// Context with only claims
	ctxWithClaims := auth.WithClaims(ctx, claims)
	retrievedClaims, ok := auth.ClaimsFromContext(ctxWithClaims)
	if !ok || retrievedClaims.UserID != userID {
		t.Errorf("failed to retrieve claims from context: %+v", retrievedClaims)
	}
	retrievedID, ok := auth.UserIDFromContext(ctxWithClaims)
	if !ok || retrievedID != userID {
		t.Errorf("expected UserIDFromContext to return %v, got %v", userID, retrievedID)
	}

	// Context with user
	user := &store.User{
		ID:       userID,
		Email:    "test@example.com",
		Timezone: "UTC",
	}
	ctxWithUser := auth.WithUser(ctx, user)
	retrievedUser, ok := auth.UserFromContext(ctxWithUser)
	if !ok || retrievedUser.ID != userID {
		t.Errorf("failed to retrieve user from context: %+v", retrievedUser)
	}
	retrievedIDFromUser, ok := auth.UserIDFromContext(ctxWithUser)
	if !ok || retrievedIDFromUser != userID {
		t.Errorf("expected UserIDFromContext from user to return %v, got %v", userID, retrievedIDFromUser)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		expected    string
		expectError bool
	}{
		{
			name:        "valid bearer token",
			header:      "Bearer my-secret-token",
			expected:    "my-secret-token",
			expectError: false,
		},
		{
			name:        "case insensitive bearer",
			header:      "bearer my-secret-token",
			expected:    "my-secret-token",
			expectError: false,
		},
		{
			name:        "bearer with extra spaces",
			header:      "Bearer   my-secret-token   ",
			expected:    "my-secret-token",
			expectError: false,
		},
		{
			name:        "empty header",
			header:      "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "basic auth format",
			header:      "Basic dXNlcjpwYXNz",
			expected:    "",
			expectError: true,
		},
		{
			name:        "bearer keyword only",
			header:      "Bearer",
			expected:    "",
			expectError: true,
		},
		{
			name:        "bearer with only spaces",
			header:      "Bearer    ",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			token, err := auth.ExtractBearerToken(req)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if token != tc.expected {
					t.Errorf("expected token %q, got %q", tc.expected, token)
				}
			}
		})
	}
}

func TestMiddlewareSuccess(t *testing.T) {
	tm := auth.NewTokenManager("secret-key-test", 1*time.Hour)
	memStore := store.NewMemoryStore()
	userID := uuid.New()
	user := &store.User{
		ID:        userID,
		Email:     "user@example.com",
		CreatedAt: time.Now().UTC(),
	}
	if err := memStore.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("failed to create user in store: %v", err)
	}

	token, err := tm.GenerateToken(userID, user.Email)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	middleware := auth.NewMiddleware(tm, memStore)

	var reachedProtected bool
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedProtected = true

		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims.UserID != userID {
			t.Errorf("claims missing or mismatch: %+v", claims)
		}

		ctxUser, ok := auth.UserFromContext(r.Context())
		if !ok || ctxUser.ID != userID {
			t.Errorf("user missing or mismatch: %+v", ctxUser)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !reachedProtected {
		t.Error("protected handler was not reached")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestMiddlewareFailures(t *testing.T) {
	tm := auth.NewTokenManager("secret-key-test", 1*time.Hour)
	memStore := store.NewMemoryStore()
	userID := uuid.New()
	user := &store.User{
		ID:        userID,
		Email:     "user@example.com",
		CreatedAt: time.Now().UTC(),
	}
	_ = memStore.CreateUser(context.Background(), user)

	validToken, _ := tm.GenerateToken(userID, user.Email)

	expiredTM := auth.NewTokenManager("secret-key-test", -1*time.Minute)
	expiredToken, _ := expiredTM.GenerateToken(userID, user.Email)

	// User not in DB
	deletedUserID := uuid.New()
	deletedUserToken, _ := tm.GenerateToken(deletedUserID, "deleted@example.com")

	middleware := auth.NewMiddleware(tm, memStore)
	dummyHandler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name          string
		authHeader    string
		expectedError string
	}{
		{
			name:          "missing authorization header",
			authHeader:    "",
			expectedError: "Missing or invalid authorization header",
		},
		{
			name:          "invalid authorization header format",
			authHeader:    "Basic 12345",
			expectedError: "Missing or invalid authorization header",
		},
		{
			name:          "invalid token format",
			authHeader:    "Bearer not.a.valid.jwt",
			expectedError: "Invalid or expired token",
		},
		{
			name:          "expired token",
			authHeader:    "Bearer " + expiredToken,
			expectedError: "Invalid or expired token",
		},
		{
			name:          "user not found in store",
			authHeader:    "Bearer " + deletedUserToken,
			expectedError: "User not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()

			dummyHandler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
			}

			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if body["error"] != tc.expectedError {
				t.Errorf("expected error %q, got %q", tc.expectedError, body["error"])
			}
		})
	}

	// Verify WrapFunc also works
	wrapFunc := middleware.WrapFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	rec := httptest.NewRecorder()
	wrapFunc(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("WrapFunc expected %d, got %d", http.StatusOK, rec.Code)
	}
}

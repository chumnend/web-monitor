package store_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"web-monitor/internal/store"
)

func runUserStoreTests(t *testing.T, s store.Store) {
	ctx := context.Background()

	// 0. Ping
	if err := s.Ping(ctx); err != nil {
		t.Fatalf("store ping failed: %v", err)
	}

	// 1. Create User
	id := uuid.New()
	email := "test-" + id.String()[:8] + "@example.com"
	user := &store.User{
		ID:           id,
		Email:        email,
		PasswordHash: "hashedpassword123",
		Timezone:     "America/Chicago",
		CreatedAt:    time.Now().Truncate(time.Microsecond).UTC(),
	}

	err := s.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 2. Fetch User By Email
	found, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}
	if found.ID != id {
		t.Errorf("expected ID %v, got %v", id, found.ID)
	}
	if found.Email != email {
		t.Errorf("expected email %v, got %v", email, found.Email)
	}
	if found.PasswordHash != user.PasswordHash {
		t.Errorf("expected password hash %v, got %v", user.PasswordHash, found.PasswordHash)
	}
	if found.Timezone != "America/Chicago" {
		t.Errorf("expected timezone America/Chicago, got %v", found.Timezone)
	}

	// Case-insensitive email fetch
	foundCase, err := s.GetUserByEmail(ctx, "TEST-"+id.String()[:8]+"@EXAMPLE.COM")
	if err != nil {
		t.Fatalf("failed to get user by uppercase email: %v", err)
	}
	if foundCase.ID != id {
		t.Errorf("case-insensitive email lookup failed: got ID %v", foundCase.ID)
	}

	// 3. Fetch User By ID
	foundByID, err := s.GetUserByID(ctx, id)
	if err != nil {
		t.Fatalf("failed to get user by ID: %v", err)
	}
	if foundByID.Email != email {
		t.Errorf("expected email %v, got %v", email, foundByID.Email)
	}

	// 4. Duplicate Email Error
	dupUser := &store.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "anotherhash",
	}
	err = s.CreateUser(ctx, dupUser)
	if err != store.ErrDuplicateEmail {
		t.Errorf("expected ErrDuplicateEmail on duplicate insert, got: %v", err)
	}

	// 5. Not Found Errors
	_, err = s.GetUserByEmail(ctx, "nonexistent@example.com")
	if err != store.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound for missing email, got: %v", err)
	}

	_, err = s.GetUserByID(ctx, uuid.New())
	if err != store.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound for missing ID, got: %v", err)
	}
}

func runRefreshTokenStoreTests(t *testing.T, s store.Store) {
	ctx := context.Background()

	// Create user first
	userID := uuid.New()
	user := &store.User{
		ID:           userID,
		Email:        "rt-" + userID.String()[:8] + "@example.com",
		PasswordHash: "passhash",
		Timezone:     "UTC",
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 1. Create Refresh Token
	tokenID := uuid.New()
	hash1 := "hash-token-1-" + tokenID.String()
	expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond).UTC()
	rt1 := &store.RefreshToken{
		ID:        tokenID,
		UserID:    userID,
		TokenHash: hash1,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().Truncate(time.Microsecond).UTC(),
	}

	if err := s.CreateRefreshToken(ctx, rt1); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}

	// 2. Get Refresh Token
	found, err := s.GetRefreshToken(ctx, hash1)
	if err != nil {
		t.Fatalf("failed to get refresh token: %v", err)
	}
	if found.ID != tokenID {
		t.Errorf("expected token ID %v, got %v", tokenID, found.ID)
	}
	if found.UserID != userID {
		t.Errorf("expected user ID %v, got %v", userID, found.UserID)
	}
	if found.RevokedAt != nil {
		t.Errorf("expected RevokedAt to be nil, got: %v", found.RevokedAt)
	}

	// 3. Revoke single token
	revokedAt := time.Now().Truncate(time.Microsecond).UTC()
	if err := s.RevokeRefreshToken(ctx, hash1, revokedAt); err != nil {
		t.Fatalf("failed to revoke refresh token: %v", err)
	}

	foundRevoked, err := s.GetRefreshToken(ctx, hash1)
	if err != nil {
		t.Fatalf("failed to get revoked token: %v", err)
	}
	if foundRevoked.RevokedAt == nil {
		t.Fatal("expected RevokedAt to be non-nil")
	}

	// 4. Create second token and revoke user tokens
	hash2 := "hash-token-2-" + uuid.New().String()
	rt2 := &store.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hash2,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().Truncate(time.Microsecond).UTC(),
	}
	if err := s.CreateRefreshToken(ctx, rt2); err != nil {
		t.Fatalf("failed to create second refresh token: %v", err)
	}

	if err := s.RevokeUserRefreshTokens(ctx, userID, revokedAt); err != nil {
		t.Fatalf("failed to revoke user refresh tokens: %v", err)
	}

	foundUserRevoked, err := s.GetRefreshToken(ctx, hash2)
	if err != nil {
		t.Fatalf("failed to get second revoked token: %v", err)
	}
	if foundUserRevoked.RevokedAt == nil {
		t.Fatal("expected second token to be revoked by user revocation")
	}

	// 5. Not found error
	_, err = s.GetRefreshToken(ctx, "nonexistent-hash")
	if err != store.ErrRefreshTokenNotFound {
		t.Errorf("expected ErrRefreshTokenNotFound, got: %v", err)
	}
}

func TestMemoryStoreUsers(t *testing.T) {
	memStore := store.NewMemoryStore()
	runUserStoreTests(t, memStore)
}

func TestMemoryStoreRefreshTokens(t *testing.T) {
	memStore := store.NewMemoryStore()
	runRefreshTokenStoreTests(t, memStore)
}

func TestPostgresStoreUsers(t *testing.T) {
	dbURL := os.Getenv("TEST_PG_URL")
	if dbURL == "" {
		dbURL = os.Getenv("PG_URL")
	}
	if dbURL == "" {
		t.Skip("skipping PostgresStore test: neither TEST_PG_URL nor PG_URL is set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("skipping PostgresStore test: could not ping database: %v", err)
	}

	pgStore := store.NewPostgresStore(db)
	runUserStoreTests(t, pgStore)
	runRefreshTokenStoreTests(t, pgStore)
}

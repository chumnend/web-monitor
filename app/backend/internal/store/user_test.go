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

func TestMemoryStoreUsers(t *testing.T) {
	memStore := store.NewMemoryStore()
	runUserStoreTests(t, memStore)
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
}

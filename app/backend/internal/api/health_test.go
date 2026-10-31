package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/api"
	"web-monitor/internal/store"
)

func TestHealthEndpoint(t *testing.T) {
	client := api.NewAPIClient()
	defer client.Close()

	testServer := httptest.NewServer(client.Handler())
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/api/health")
	if err != nil {
		t.Fatalf("Failed to make GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}

	// 1. Verify MessageResponse compatibility
	var msgBody api.MessageResponse
	// Re-read with HealthResponse
	var healthBody api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthBody); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}
	msgBody.Message = healthBody.Message

	if msgBody.Message != "OK" {
		t.Errorf("Expected message 'OK', got '%s'", msgBody.Message)
	}

	// 2. Verify enriched HealthResponse fields
	if healthBody.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", healthBody.Status)
	}

	if healthBody.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if time.Since(healthBody.Timestamp) > 10*time.Second {
		t.Errorf("Timestamp too old: %v", healthBody.Timestamp)
	}

	if healthBody.Services == nil || healthBody.Services["database"] != "healthy" {
		t.Errorf("Expected database service healthy, got: %+v", healthBody.Services)
	}
}

// failingStore wraps a Store and fails Ping
type failingStore struct {
	store.Store
}

func (f *failingStore) Ping(_ context.Context) error {
	return errors.New("connection refused")
}

func (f *failingStore) CreateUser(ctx context.Context, user *store.User) error {
	return f.Store.CreateUser(ctx, user)
}

func (f *failingStore) GetUserByEmail(ctx context.Context, email string) (*store.User, error) {
	return f.Store.GetUserByEmail(ctx, email)
}

func (f *failingStore) GetUserByID(ctx context.Context, id uuid.UUID) (*store.User, error) {
	return f.Store.GetUserByID(ctx, id)
}

func (f *failingStore) CreateRefreshToken(ctx context.Context, token *store.RefreshToken) error {
	return f.Store.CreateRefreshToken(ctx, token)
}

func (f *failingStore) GetRefreshToken(ctx context.Context, tokenHash string) (*store.RefreshToken, error) {
	return f.Store.GetRefreshToken(ctx, tokenHash)
}

func (f *failingStore) RevokeRefreshToken(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	return f.Store.RevokeRefreshToken(ctx, tokenHash, revokedAt)
}

func (f *failingStore) RevokeUserRefreshTokens(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	return f.Store.RevokeUserRefreshTokens(ctx, userID, revokedAt)
}

func TestHealthEndpointUnhealthyDatabase(t *testing.T) {
	badStore := &failingStore{Store: store.NewMemoryStore()}
	client := api.NewAPIClient(api.WithStore(badStore))
	defer client.Close()

	testServer := httptest.NewServer(client.Handler())
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/api/health")
	if err != nil {
		t.Fatalf("Failed to make GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status code 503, got %d", resp.StatusCode)
	}

	var healthBody api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthBody); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if healthBody.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got '%s'", healthBody.Status)
	}

	if !strings.Contains(healthBody.Services["database"], "connection refused") {
		t.Errorf("Expected error details in database check, got: %s", healthBody.Services["database"])
	}
}

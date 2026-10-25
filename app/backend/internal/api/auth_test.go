package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"web-monitor/internal/api"
	"web-monitor/internal/auth"
	"web-monitor/internal/store"
)

func TestRegisterSuccess(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	payload := map[string]string{
		"email":    "alice@example.com",
		"password": "SuperSecretPassword123",
		"timezone": "America/New_York",
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/auth/register", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var res api.RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.User.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", res.User.Email)
	}
	if res.User.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", res.User.Timezone)
	}
	if res.User.ID.String() == "" {
		t.Error("expected non-empty user ID")
	}
	if res.Message != "User registered successfully" {
		t.Errorf("expected success message, got %s", res.Message)
	}

	// Verify the user is stored in the database with a valid bcrypt hash
	storedUser, err := memStore.GetUserByEmail(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatalf("failed to retrieve stored user: %v", err)
	}
	if err := auth.ComparePassword(storedUser.PasswordHash, "SuperSecretPassword123"); err != nil {
		t.Errorf("stored password hash does not match plaintext: %v", err)
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	tests := []struct {
		name           string
		contentType    string
		payload        string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid json",
			contentType:    "application/json",
			payload:        `{bad-json}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
		{
			name:           "invalid content type",
			contentType:    "text/plain",
			payload:        `{"email":"a@b.com","password":"password123"}`,
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "Content-Type must be application/json",
		},
		{
			name:           "invalid email",
			contentType:    "application/json",
			payload:        `{"email":"not-an-email","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid email address",
		},
		{
			name:           "password too short",
			contentType:    "application/json",
			payload:        `{"email":"valid@example.com","password":"short"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Password must be at least 8 characters long",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, server.URL+"/api/auth/register", bytes.NewBufferString(tc.payload))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", tc.contentType)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to execute request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			var errRes api.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errRes); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if errRes.Error != tc.expectedError {
				t.Errorf("expected error %q, got %q", tc.expectedError, errRes.Error)
			}
		})
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	payload := `{"email":"duplicate@example.com","password":"password123"}`

	// First registration - success
	resp1, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("first registration request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for first registration, got %d", resp1.StatusCode)
	}

	// Second registration with same email - conflict
	resp2, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("second registration request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate registration, got %d", resp2.StatusCode)
	}

	var errRes api.ErrorResponse
	if err := json.NewDecoder(resp2.Body).Decode(&errRes); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errRes.Error != "User with this email already exists" {
		t.Errorf("expected duplicate user message, got %s", errRes.Error)
	}
}

func TestLoginSuccess(t *testing.T) {
	memStore := store.NewMemoryStore()
	secret := "custom-jwt-secret-for-login-test"
	client := api.NewAPIClient(api.WithStore(memStore), api.WithSecretKey(secret))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	// Register user first
	regPayload := `{"email":"bob@example.com","password":"Password123!","timezone":"UTC"}`
	regResp, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewBufferString(regPayload))
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	regResp.Body.Close()
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on registration, got %d", regResp.StatusCode)
	}

	// Login with uppercase email (case-insensitive)
	loginPayload := `{"email":"BOB@EXAMPLE.COM","password":"Password123!"}`
	loginResp, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewBufferString(loginPayload))
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", loginResp.StatusCode)
	}

	var res api.LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	if res.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if res.User.Email != "bob@example.com" {
		t.Errorf("expected user email bob@example.com, got %s", res.User.Email)
	}
	if res.ExpiresIn <= 0 {
		t.Errorf("expected positive expires_in, got %d", res.ExpiresIn)
	}

	// Validate the JWT token using TokenManager
	tm := auth.NewTokenManager(secret, 24*time.Hour)
	claims, err := tm.ValidateToken(res.Token)
	if err != nil {
		t.Fatalf("token validation failed: %v", err)
	}
	if claims.Email != "bob@example.com" {
		t.Errorf("expected claims email bob@example.com, got %s", claims.Email)
	}
	if claims.UserID != res.User.ID {
		t.Errorf("expected claims user_id %v, got %v", res.User.ID, claims.UserID)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	// Register a user
	regPayload := `{"email":"carol@example.com","password":"Password123!"}`
	regResp, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewBufferString(regPayload))
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	regResp.Body.Close()

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{
			name:     "wrong password",
			email:    "carol@example.com",
			password: "WrongPassword!",
		},
		{
			name:     "unregistered email",
			email:    "unknown@example.com",
			password: "Password123!",
		},
		{
			name:     "empty email",
			email:    "",
			password: "Password123!",
		},
		{
			name:     "empty password",
			email:    "carol@example.com",
			password: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]string{
				"email":    tc.email,
				"password": tc.password,
			})
			resp, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(payload))
			if err != nil {
				t.Fatalf("login request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
			}

			var errRes api.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errRes); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if errRes.Error != "Invalid email or password" {
				t.Errorf("expected 'Invalid email or password', got %s", errRes.Error)
			}
		})
	}
}

func TestLoginValidationErrors(t *testing.T) {
	client := api.NewAPIClient()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	// Malformed JSON
	resp, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewBufferString(`{invalid-json}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}

	// Unsupported Content-Type
	resp2, err := http.Post(server.URL+"/api/auth/login", "text/plain", bytes.NewBufferString(`{"email":"a@b.com","password":"p"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415 Unsupported Media Type, got %d", resp2.StatusCode)
	}
}

func TestCorsPreflight(t *testing.T) {
	client := api.NewAPIClient()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	req, err := http.NewRequest(http.MethodOptions, server.URL+"/api/auth/register", nil)
	if err != nil {
		t.Fatalf("failed to create options request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute options request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d for OPTIONS, got %d", http.StatusNoContent, resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got %s", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

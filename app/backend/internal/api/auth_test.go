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

func TestMeEndpointSuccess(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore), api.WithSecretKey("test-secret-key"))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	// Register user
	regPayload := `{"email":"david@example.com","password":"Password123!","timezone":"America/New_York"}`
	regResp, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewBufferString(regPayload))
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	regResp.Body.Close()

	// Login
	loginPayload := `{"email":"david@example.com","password":"Password123!"}`
	loginResp, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewBufferString(loginPayload))
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	defer loginResp.Body.Close()

	var loginData api.LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginData); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	// Fetch /api/auth/me with Bearer token
	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+loginData.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var userRes api.UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userRes); err != nil {
		t.Fatalf("failed to decode user response: %v", err)
	}

	if userRes.Email != "david@example.com" {
		t.Errorf("expected email david@example.com, got %s", userRes.Email)
	}
	if userRes.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", userRes.Timezone)
	}
	if userRes.ID != loginData.User.ID {
		t.Errorf("expected user ID %v, got %v", loginData.User.ID, userRes.ID)
	}
}

func TestMeEndpointUnauthorized(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

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
			name:          "invalid format",
			authHeader:    "Token 12345",
			expectedError: "Missing or invalid authorization header",
		},
		{
			name:          "invalid token",
			authHeader:    "Bearer invalid.token.value",
			expectedError: "Invalid or expired token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to execute request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", resp.StatusCode)
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

func TestRefreshEndpointSuccess(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	// Register user
	regPayload := `{"email":"emma@example.com","password":"Password123!"}`
	regResp, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewBufferString(regPayload))
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	regResp.Body.Close()

	// Login
	loginPayload := `{"email":"emma@example.com","password":"Password123!"}`
	loginResp, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewBufferString(loginPayload))
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	defer loginResp.Body.Close()

	var loginData api.LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginData); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginData.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token from login")
	}

	// Refresh token
	refPayload, _ := json.Marshal(api.RefreshRequest{
		RefreshToken: loginData.RefreshToken,
	})
	refResp, err := http.Post(server.URL+"/api/auth/refresh", "application/json", bytes.NewReader(refPayload))
	if err != nil {
		t.Fatalf("failed to refresh token: %v", err)
	}
	defer refResp.Body.Close()

	if refResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from refresh, got %d", refResp.StatusCode)
	}

	var refData api.RefreshResponse
	if err := json.NewDecoder(refResp.Body).Decode(&refData); err != nil {
		t.Fatalf("failed to decode refresh response: %v", err)
	}

	if refData.Token == "" {
		t.Fatal("expected non-empty access token")
	}
	if refData.RefreshToken == "" {
		t.Fatal("expected non-empty rotated refresh token")
	}
	if refData.RefreshToken == loginData.RefreshToken {
		t.Fatal("refresh token must rotate")
	}
	if refData.User.Email != "emma@example.com" {
		t.Errorf("expected email emma@example.com, got %s", refData.User.Email)
	}

	// Verify the new access token can be used on a protected endpoint
	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
	if err != nil {
		t.Fatalf("failed to create me request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+refData.Token)
	meResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to call me endpoint: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK with new access token, got %d", meResp.StatusCode)
	}
}

func TestRefreshEndpointReuseRevocation(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	// Register user
	regPayload := `{"email":"frank@example.com","password":"Password123!"}`
	regResp, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewBufferString(regPayload))
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}
	regResp.Body.Close()

	// Login
	loginPayload := `{"email":"frank@example.com","password":"Password123!"}`
	loginResp, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewBufferString(loginPayload))
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	defer loginResp.Body.Close()

	var loginData api.LoginResponse
	_ = json.NewDecoder(loginResp.Body).Decode(&loginData)

	// First refresh -> success
	refPayload, _ := json.Marshal(api.RefreshRequest{
		RefreshToken: loginData.RefreshToken,
	})
	refResp1, err := http.Post(server.URL+"/api/auth/refresh", "application/json", bytes.NewReader(refPayload))
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	defer refResp1.Body.Close()
	if refResp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", refResp1.StatusCode)
	}

	var refData1 api.RefreshResponse
	_ = json.NewDecoder(refResp1.Body).Decode(&refData1)

	// Second refresh using old token -> 401 Unauthorized (and revokes remaining tokens for user)
	refResp2, err := http.Post(server.URL+"/api/auth/refresh", "application/json", bytes.NewReader(refPayload))
	if err != nil {
		t.Fatalf("second refresh failed: %v", err)
	}
	defer refResp2.Body.Close()
	if refResp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for reused refresh token, got %d", refResp2.StatusCode)
	}

	// Attempt using rotated token also fails because reuse revocation invalidated all user tokens
	refPayloadRotated, _ := json.Marshal(api.RefreshRequest{
		RefreshToken: refData1.RefreshToken,
	})
	refResp3, err := http.Post(server.URL+"/api/auth/refresh", "application/json", bytes.NewReader(refPayloadRotated))
	if err != nil {
		t.Fatalf("third refresh failed: %v", err)
	}
	defer refResp3.Body.Close()
	if refResp3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on token after theft revocation, got %d", refResp3.StatusCode)
	}
}

func TestRefreshEndpointInvalid(t *testing.T) {
	client := api.NewAPIClient()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	// Missing refresh_token
	resp1, err := http.Post(server.URL+"/api/auth/refresh", "application/json", bytes.NewBufferString(`{"refresh_token":""}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp1.StatusCode)
	}

	// Nonexistent refresh_token
	resp2, err := http.Post(server.URL+"/api/auth/refresh", "application/json", bytes.NewBufferString(`{"refresh_token":"nonexistent-token"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp2.StatusCode)
	}

	// Unsupported Media Type
	resp3, err := http.Post(server.URL+"/api/auth/refresh", "text/plain", bytes.NewBufferString(`token`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415 Unsupported Media Type, got %d", resp3.StatusCode)
	}
}

func TestAuthRateLimiting(t *testing.T) {
	memStore := store.NewMemoryStore()
	limiter := auth.NewRateLimiter(2, 1*time.Minute)
	defer limiter.Close()

	client := api.NewAPIClient(api.WithStore(memStore), api.WithRateLimiter(limiter))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	loginPayload := `{"email":"test@example.com","password":"badpassword"}`

	// 1st request -> 401 Unauthorized (allowed by rate limiter)
	resp1, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewBufferString(loginPayload))
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("req 1: expected 401, got %d", resp1.StatusCode)
	}
	if resp1.Header.Get("X-RateLimit-Remaining") != "1" {
		t.Errorf("req 1: expected remaining 1, got %s", resp1.Header.Get("X-RateLimit-Remaining"))
	}

	// 2nd request -> 401 Unauthorized (allowed by rate limiter)
	resp2, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewBufferString(loginPayload))
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("req 2: expected 401, got %d", resp2.StatusCode)
	}
	if resp2.Header.Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("req 2: expected remaining 0, got %s", resp2.Header.Get("X-RateLimit-Remaining"))
	}

	// 3rd request -> 429 Too Many Requests
	resp3, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewBufferString(loginPayload))
	if err != nil {
		t.Fatalf("third request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusTooManyRequests {
		t.Errorf("req 3: expected 429, got %d", resp3.StatusCode)
	}
	if resp3.Header.Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}

	var errRes api.ErrorResponse
	if err := json.NewDecoder(resp3.Body).Decode(&errRes); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if errRes.Error != "Too many requests, please try again later" {
		t.Errorf("unexpected error message: %s", errRes.Error)
	}
}

func TestFullAuthFlowLifecycle(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore), api.WithSecretKey("super-secret-lifecycle-test-key"))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	email := "lifecycle@example.com"
	password := "SecurePass123!"

	// Step 1: Registration
	regBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
		"timezone": "Europe/London",
	})
	regResp, err := http.Post(server.URL+"/api/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("step 1 register failed: %v", err)
	}
	defer regResp.Body.Close()
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("step 1 register status %d", regResp.StatusCode)
	}

	// Step 2: Login
	loginBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	loginResp, err := http.Post(server.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("step 2 login failed: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("step 2 login status %d", loginResp.StatusCode)
	}

	var loginData api.LoginResponse
	_ = json.NewDecoder(loginResp.Body).Decode(&loginData)
	if loginData.Token == "" || loginData.RefreshToken == "" {
		t.Fatal("expected both access token and refresh token")
	}

	// Step 3: Access protected endpoint /api/auth/me
	meReq, _ := http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginData.Token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatalf("step 3 me failed: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("step 3 me status %d", meResp.StatusCode)
	}

	var meData api.UserResponse
	_ = json.NewDecoder(meResp.Body).Decode(&meData)
	if meData.Email != email || meData.Timezone != "Europe/London" {
		t.Errorf("step 3 me data mismatch: %+v", meData)
	}

	// Step 4: Refresh token
	refBody, _ := json.Marshal(api.RefreshRequest{
		RefreshToken: loginData.RefreshToken,
	})
	refResp, err := http.Post(server.URL+"/api/auth/refresh", "application/json", bytes.NewReader(refBody))
	if err != nil {
		t.Fatalf("step 4 refresh failed: %v", err)
	}
	defer refResp.Body.Close()
	if refResp.StatusCode != http.StatusOK {
		t.Fatalf("step 4 refresh status %d", refResp.StatusCode)
	}

	var refData api.RefreshResponse
	_ = json.NewDecoder(refResp.Body).Decode(&refData)
	if refData.Token == "" || refData.RefreshToken == "" {
		t.Fatal("expected new tokens from refresh")
	}
	if refData.RefreshToken == loginData.RefreshToken {
		t.Fatal("refresh token must rotate")
	}

	// Step 5: Verify new access token works on /api/auth/me
	meReq2, _ := http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
	meReq2.Header.Set("Authorization", "Bearer "+refData.Token)
	meResp2, err := http.DefaultClient.Do(meReq2)
	if err != nil {
		t.Fatalf("step 5 me with refreshed token failed: %v", err)
	}
	defer meResp2.Body.Close()
	if meResp2.StatusCode != http.StatusOK {
		t.Fatalf("step 5 me with refreshed token status %d", meResp2.StatusCode)
	}

	// Step 6: Old refresh token rejected (revoked by rotation)
	oldRefResp, err := http.Post(server.URL+"/api/auth/refresh", "application/json", bytes.NewReader(refBody))
	if err != nil {
		t.Fatalf("step 6 old refresh request failed: %v", err)
	}
	defer oldRefResp.Body.Close()
	if oldRefResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("step 6 expected 401 for old refresh token, got %d", oldRefResp.StatusCode)
	}
}




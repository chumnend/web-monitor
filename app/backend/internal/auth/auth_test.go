package auth_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"web-monitor/internal/auth"
	"web-monitor/internal/store"
)

func TestHashPasswordAndCompare(t *testing.T) {
	password := "SecretPass123!"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}

	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	if hash == password {
		t.Fatal("hash should not match plain password")
	}

	// Verify with bcrypt directly
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		t.Fatalf("bcrypt verification failed: %v", err)
	}

	// Verify using helper
	if err := auth.ComparePassword(hash, password); err != nil {
		t.Fatalf("ComparePassword failed for correct password: %v", err)
	}

	// Verify incorrect password fails
	if err := auth.ComparePassword(hash, "WrongPassword"); err == nil {
		t.Fatal("ComparePassword should fail for wrong password")
	}
}

func TestPasswordLengthConstraints(t *testing.T) {
	// Too short (< 8 chars)
	_, err := auth.HashPassword("short")
	if err != auth.ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort, got: %v", err)
	}

	// 7 characters (boundary)
	_, err = auth.HashPassword("1234567")
	if err != auth.ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort for 7 chars, got: %v", err)
	}

	// 8 characters (valid boundary)
	_, err = auth.HashPassword("12345678")
	if err != nil {
		t.Fatalf("expected 8 chars to be valid, got: %v", err)
	}

	// 72 characters (valid boundary)
	pass72 := strings.Repeat("a", 72)
	_, err = auth.HashPassword(pass72)
	if err != nil {
		t.Fatalf("expected 72 chars to be valid, got: %v", err)
	}

	// 73 characters (too long)
	pass73 := strings.Repeat("a", 73)
	_, err = auth.HashPassword(pass73)
	if err != auth.ErrPasswordTooLong {
		t.Fatalf("expected ErrPasswordTooLong for 73 chars, got: %v", err)
	}
}

func TestValidateAndNormalizeEmail(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "valid simple email",
			input:       "user@example.com",
			expected:    "user@example.com",
			expectError: false,
		},
		{
			name:        "trimmed and lowercased",
			input:       "   User.Name@Example.COM   ",
			expected:    "user.name@example.com",
			expectError: false,
		},
		{
			name:        "empty email",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "spaces only",
			input:       "    ",
			expected:    "",
			expectError: true,
		},
		{
			name:        "missing @",
			input:       "userexample.com",
			expected:    "",
			expectError: true,
		},
		{
			name:        "missing domain dot",
			input:       "user@localhost",
			expected:    "",
			expectError: true,
		},
		{
			name:        "display name format",
			input:       "User <user@example.com>",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normalized, err := auth.ValidateAndNormalizeEmail(tc.input)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tc.input, err)
				}
				if normalized != tc.expected {
					t.Errorf("expected %q, got %q", tc.expected, normalized)
				}
			}
		})
	}
}

func TestNormalizeTimezone(t *testing.T) {
	if tz := auth.NormalizeTimezone("America/New_York"); tz != "America/New_York" {
		t.Errorf("expected America/New_York, got %s", tz)
	}

	if tz := auth.NormalizeTimezone("   "); tz != "UTC" {
		t.Errorf("expected UTC for empty timezone, got %s", tz)
	}

	if tz := auth.NormalizeTimezone("Invalid/Timezone"); tz != "UTC" {
		t.Errorf("expected UTC for invalid timezone, got %s", tz)
	}
}

func TestTokenManager(t *testing.T) {
	secret := "test-secret-key-123"
	duration := 1 * time.Hour
	tm := auth.NewTokenManager(secret, duration)

	userID := uuid.New()
	email := "tokenuser@example.com"

	// 1. Generate valid token
	tokenStr, err := tm.GenerateToken(userID, email)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token string")
	}

	// 2. Validate token
	claims, err := tm.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("unexpected error validating token: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("expected userID %v, got %v", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("expected email %v, got %v", email, claims.Email)
	}
	if claims.Subject != userID.String() {
		t.Errorf("expected subject %v, got %v", userID.String(), claims.Subject)
	}

	// 3. Validate with wrong secret fails
	wrongTM := auth.NewTokenManager("different-secret", duration)
	_, err = wrongTM.ValidateToken(tokenStr)
	if err != auth.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken with wrong secret, got: %v", err)
	}

	// 4. Expired token fails
	expiredTM := auth.NewTokenManager(secret, -1*time.Minute)
	expiredToken, err := expiredTM.GenerateToken(userID, email)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}
	_, err = tm.ValidateToken(expiredToken)
	if err != auth.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for expired token, got: %v", err)
	}

	// 5. Tampered token fails
	tampered := tokenStr + "tamper"
	_, err = tm.ValidateToken(tampered)
	if err != auth.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for tampered token, got: %v", err)
	}

	// 6. Non-HMAC signing method rejected
	unsecuredToken := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	noneStr, _ := unsecuredToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	_, err = tm.ValidateToken(noneStr)
	if err != auth.ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for none algorithm, got: %v", err)
	}
}

func TestServiceRegisterAndLogin(t *testing.T) {
	memStore := store.NewMemoryStore()
	tm := auth.NewTokenManager("test-jwt-secret", 2*time.Hour)
	svc := auth.NewService(memStore, auth.WithTokenManager(tm))
	ctx := context.Background()

	// 1. Successful Registration
	input := auth.RegisterInput{
		Email:    "  NewUser@Example.Com  ",
		Password: "SecurePassword123!",
		Timezone: "America/Chicago",
	}

	user, err := svc.Register(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error registering user: %v", err)
	}

	if user.Email != "newuser@example.com" {
		t.Errorf("expected normalized email newuser@example.com, got %s", user.Email)
	}

	// 2. Successful Login
	loginRes, err := svc.Login(ctx, auth.LoginInput{
		Email:    "NEWUSER@EXAMPLE.COM", // case-insensitive check
		Password: "SecurePassword123!",
	})
	if err != nil {
		t.Fatalf("unexpected error during login: %v", err)
	}
	if loginRes.Token == "" {
		t.Fatal("expected non-empty token from login")
	}
	if loginRes.User.ID != user.ID {
		t.Errorf("expected user ID %v, got %v", user.ID, loginRes.User.ID)
	}
	if loginRes.ExpiresIn != int64(2*3600) {
		t.Errorf("expected expires_in %d, got %d", 2*3600, loginRes.ExpiresIn)
	}

	// Verify returned token is valid and decodes to correct claims
	claims, err := tm.ValidateToken(loginRes.Token)
	if err != nil {
		t.Fatalf("login token validation failed: %v", err)
	}
	if claims.UserID != user.ID || claims.Email != user.Email {
		t.Errorf("claims mismatch: %+v", claims)
	}

	// 3. Login with wrong password
	_, err = svc.Login(ctx, auth.LoginInput{
		Email:    "newuser@example.com",
		Password: "WrongPassword!",
	})
	if err != auth.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}

	// 4. Login with non-existent email
	_, err = svc.Login(ctx, auth.LoginInput{
		Email:    "nonexistent@example.com",
		Password: "SecurePassword123!",
	})
	if err != auth.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for nonexistent email, got: %v", err)
	}

	// 5. Login with empty email or password
	_, err = svc.Login(ctx, auth.LoginInput{
		Email:    "",
		Password: "SecurePassword123!",
	})
	if err != auth.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for empty email, got: %v", err)
	}

	_, err = svc.Login(ctx, auth.LoginInput{
		Email:    "newuser@example.com",
		Password: "",
	})
	if err != auth.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for empty password, got: %v", err)
	}
}

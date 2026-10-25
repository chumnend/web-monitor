package auth_test

import (
	"context"
	"strings"
	"testing"

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

func TestServiceRegister(t *testing.T) {
	memStore := store.NewMemoryStore()
	svc := auth.NewService(memStore)
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
	if user.Timezone != "America/Chicago" {
		t.Errorf("expected timezone America/Chicago, got %s", user.Timezone)
	}
	if user.PasswordHash == "" || user.PasswordHash == input.Password {
		t.Errorf("invalid password hash: %s", user.PasswordHash)
	}
	if err := auth.ComparePassword(user.PasswordHash, "SecurePassword123!"); err != nil {
		t.Errorf("password hash does not match original password: %v", err)
	}

	// 2. Duplicate Email Registration
	_, err = svc.Register(ctx, auth.RegisterInput{
		Email:    "newuser@example.com",
		Password: "DifferentPassword123!",
	})
	if err != auth.ErrUserAlreadyExists {
		t.Fatalf("expected ErrUserAlreadyExists, got: %v", err)
	}

	// 3. Invalid Email
	_, err = svc.Register(ctx, auth.RegisterInput{
		Email:    "invalid-email",
		Password: "SecurePassword123!",
	})
	if err != auth.ErrInvalidEmail {
		t.Fatalf("expected ErrInvalidEmail, got: %v", err)
	}

	// 4. Invalid Password (too short)
	_, err = svc.Register(ctx, auth.RegisterInput{
		Email:    "valid@example.com",
		Password: "short",
	})
	if err != auth.ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort, got: %v", err)
	}

	// 5. Default Timezone when empty
	defaultTzUser, err := svc.Register(ctx, auth.RegisterInput{
		Email:    "another@example.com",
		Password: "SecurePassword123!",
		Timezone: "",
	})
	if err != nil {
		t.Fatalf("unexpected error registering user with empty timezone: %v", err)
	}
	if defaultTzUser.Timezone != "UTC" {
		t.Errorf("expected default timezone UTC, got %s", defaultTzUser.Timezone)
	}
}

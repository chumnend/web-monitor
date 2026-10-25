package auth

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrInvalidEmail = errors.New("invalid email address")
)

// ValidateAndNormalizeEmail validates an email address and returns it normalized (trimmed and lowercase).
func ValidateAndNormalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 254 {
		return "", ErrInvalidEmail
	}

	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", ErrInvalidEmail
	}

	// Ensure there are no display name wrappers (e.g. "Name <email@example.com>")
	if addr.Address != email {
		return "", ErrInvalidEmail
	}

	// Ensure domain has at least one dot and no trailing/leading dots
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return "", ErrInvalidEmail
	}
	domain := parts[1]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return "", ErrInvalidEmail
	}

	return strings.ToLower(addr.Address), nil
}

// NormalizeTimezone checks and normalizes a timezone string, defaulting to "UTC".
func NormalizeTimezone(tz string) string {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return "UTC"
	}
	// Verify if it is a valid location in the IANA timezone database
	if _, err := time.LoadLocation(tz); err != nil {
		return "UTC"
	}
	return tz
}

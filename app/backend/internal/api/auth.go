package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/auth"
)

// RegisterRequest defines the expected JSON payload for user registration.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Timezone string `json:"timezone,omitempty"`
}

// UserResponse represents a user safe for public API responses (no password hash).
type UserResponse struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Timezone        string     `json:"timezone"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// RegisterResponse represents the response body returned upon successful registration.
type RegisterResponse struct {
	User    UserResponse `json:"user"`
	Message string       `json:"message"`
}

// ErrorResponse represents a standardized API error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func (client *APIClient) handleRegister(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	user, err := client.authService.Register(r.Context(), auth.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Timezone: req.Timezone,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidEmail):
			writeError(w, http.StatusBadRequest, "Invalid email address")
		case errors.Is(err, auth.ErrPasswordTooShort):
			writeError(w, http.StatusBadRequest, "Password must be at least 8 characters long")
		case errors.Is(err, auth.ErrPasswordTooLong):
			writeError(w, http.StatusBadRequest, "Password must not exceed 72 characters")
		case errors.Is(err, auth.ErrUserAlreadyExists):
			writeError(w, http.StatusConflict, "User with this email already exists")
		default:
			writeError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, RegisterResponse{
		User: UserResponse{
			ID:              user.ID,
			Email:           user.Email,
			Timezone:        user.Timezone,
			EmailVerifiedAt: user.EmailVerifiedAt,
			CreatedAt:       user.CreatedAt,
		},
		Message: "User registered successfully",
	})
}

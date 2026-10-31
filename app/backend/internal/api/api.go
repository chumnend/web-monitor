package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"web-monitor/internal/auth"
	"web-monitor/internal/store"
)

type APIClient struct {
	store          store.Store
	authService    *auth.Service
	authMiddleware *auth.Middleware
	rateLimiter    *auth.RateLimiter
}

type clientConfig struct {
	store          store.Store
	authService    *auth.Service
	authMiddleware *auth.Middleware
	rateLimiter    *auth.RateLimiter
	secretKey      string
}

type Option func(*clientConfig)

// WithStore sets the store for the APIClient.
func WithStore(s store.Store) Option {
	return func(cfg *clientConfig) {
		cfg.store = s
	}
}

// WithAuthService sets a custom auth service for the APIClient.
func WithAuthService(svc *auth.Service) Option {
	return func(cfg *clientConfig) {
		cfg.authService = svc
	}
}

// WithAuthMiddleware sets custom auth middleware for the APIClient.
func WithAuthMiddleware(m *auth.Middleware) Option {
	return func(cfg *clientConfig) {
		cfg.authMiddleware = m
	}
}

// WithRateLimiter sets a custom rate limiter for the APIClient.
func WithRateLimiter(rl *auth.RateLimiter) Option {
	return func(cfg *clientConfig) {
		cfg.rateLimiter = rl
	}
}

// WithSecretKey configures the JWT secret key for token issuance.
func WithSecretKey(secretKey string) Option {
	return func(cfg *clientConfig) {
		cfg.secretKey = secretKey
	}
}

func NewAPIClient(opts ...Option) *APIClient {
	cfg := &clientConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.store == nil {
		cfg.store = store.NewMemoryStore()
	}

	if cfg.authService == nil {
		var svcOpts []func(*auth.Service)
		if cfg.secretKey != "" {
			svcOpts = append(svcOpts, auth.WithTokenManager(auth.NewTokenManager(cfg.secretKey, 24*time.Hour)))
		}
		cfg.authService = auth.NewService(cfg.store, svcOpts...)
	}

	if cfg.authMiddleware == nil {
		cfg.authMiddleware = cfg.authService.Middleware()
	}

	if cfg.rateLimiter == nil {
		cfg.rateLimiter = auth.NewRateLimiter(60, time.Minute)
	}

	return &APIClient{
		store:          cfg.store,
		authService:    cfg.authService,
		authMiddleware: cfg.authMiddleware,
		rateLimiter:    cfg.rateLimiter,
	}
}

// Close releases resources held by APIClient (e.g. background janitors).
func (client *APIClient) Close() {
	if client.rateLimiter != nil {
		client.rateLimiter.Close()
	}
}

// RequireAuth wraps an http.Handler with JWT authentication middleware.
func (client *APIClient) RequireAuth(next http.Handler) http.Handler {
	return client.authMiddleware.Handler(next)
}

// RequireAuthFunc wraps an http.HandlerFunc with JWT authentication middleware.
func (client *APIClient) RequireAuthFunc(fn http.HandlerFunc) http.HandlerFunc {
	return client.authMiddleware.WrapFunc(fn)
}

// RateLimit wraps an http.Handler with rate limiting middleware.
func (client *APIClient) RateLimit(next http.Handler) http.Handler {
	if client.rateLimiter == nil {
		return next
	}
	return client.rateLimiter.Middleware(next)
}

func (client *APIClient) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (client *APIClient) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", sayHello)
	mux.Handle("POST /api/auth/register", client.RateLimit(http.HandlerFunc(client.handleRegister)))
	mux.Handle("POST /api/auth/login", client.RateLimit(http.HandlerFunc(client.handleLogin)))
	mux.Handle("POST /api/auth/refresh", client.RateLimit(http.HandlerFunc(client.handleRefresh)))
	mux.Handle("GET /api/auth/me", client.RequireAuth(http.HandlerFunc(client.handleMe)))
	return client.corsMiddleware(mux)
}

func (client APIClient) Start(port string) error {
	fmt.Printf("Server starting on port %s...\n", port)
	return http.ListenAndServe(":"+port, client.Handler())
}

type MessageResponse struct {
	Message string `json:"message"`
}

func sayHello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := MessageResponse{Message: "OK"}
	json.NewEncoder(w).Encode(response)
}

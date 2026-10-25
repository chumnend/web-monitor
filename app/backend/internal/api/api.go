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
	store       store.Store
	authService *auth.Service
}

type clientConfig struct {
	store       store.Store
	authService *auth.Service
	secretKey   string
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

	return &APIClient{
		store:       cfg.store,
		authService: cfg.authService,
	}
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
	mux.HandleFunc("POST /api/auth/register", client.handleRegister)
	mux.HandleFunc("POST /api/auth/login", client.handleLogin)
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

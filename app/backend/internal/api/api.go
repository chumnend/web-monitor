package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"web-monitor/internal/auth"
	"web-monitor/internal/store"
)

type APIClient struct {
	store       store.Store
	authService *auth.Service
}

type Option func(*APIClient)

// WithStore sets the store and creates an auth service for the APIClient.
func WithStore(s store.Store) Option {
	return func(c *APIClient) {
		c.store = s
		c.authService = auth.NewService(s)
	}
}

// WithAuthService sets a custom auth service for the APIClient.
func WithAuthService(svc *auth.Service) Option {
	return func(c *APIClient) {
		c.authService = svc
	}
}

func NewAPIClient(opts ...Option) *APIClient {
	client := &APIClient{}
	for _, opt := range opts {
		opt(client)
	}
	if client.store == nil {
		client.store = store.NewMemoryStore()
	}
	if client.authService == nil {
		client.authService = auth.NewService(client.store)
	}
	return client
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

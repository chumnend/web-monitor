package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type APIClient struct{}

func NewAPIClient() *APIClient {
	client := &APIClient{}
	return client
}

func (client *APIClient) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", sayHello)
	return mux
}

func (client APIClient) Start(port string) error {
	fmt.Printf("Server starting on port %s...\n", port)
	return http.ListenAndServe(":"+port, client.Handler())
}

type MessageResponse struct {
	Message string `json:"message"`
}

func sayHello(w http.ResponseWriter, r *http.Request) {
	// Allow access from any origin if needed, or specify the frontend origin.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	response := MessageResponse{Message: "OK"}
	json.NewEncoder(w).Encode(response)
}

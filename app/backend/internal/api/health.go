package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// MessageResponse represents a basic message response (kept for backwards compatibility).
type MessageResponse struct {
	Message string `json:"message"`
}

// HealthResponse represents a structured health check response.
type HealthResponse struct {
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    string            `json:"uptime,omitempty"`
	Services  map[string]string `json:"services,omitempty"`
}

// handleHealth handles the health check endpoint GET /api/health.
func (client *APIClient) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	services := make(map[string]string)
	isHealthy := true

	if client.store != nil {
		if err := client.store.Ping(ctx); err != nil {
			isHealthy = false
			services["database"] = "unhealthy: " + err.Error()
		} else {
			services["database"] = "healthy"
		}
	}

	var uptime string
	if !client.startTime.IsZero() {
		uptime = time.Since(client.startTime).Round(time.Second).String()
	}

	response := HealthResponse{
		Timestamp: time.Now().UTC(),
		Uptime:    uptime,
		Services:  services,
	}

	if isHealthy {
		response.Status = "healthy"
		response.Message = "OK"
		w.WriteHeader(http.StatusOK)
	} else {
		response.Status = "unhealthy"
		response.Message = "One or more dependencies are unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(response)
}

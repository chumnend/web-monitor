package api_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"web-monitor/internal/api"
)

// getFreePort finds an available TCP port for testing
func getFreePort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port), nil
}

func TestStartError(t *testing.T) {
	client := api.NewAPIClient()

	// Get a free port
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	// Start a dummy server on the chosen port to simulate it being in use
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatalf("Failed to start dummy listener on port %s: %v", port, err)
	}
	defer listener.Close() // Ensure the dummy listener is closed

	errChan := make(chan error, 1)

	// Run Start in a goroutine
	go func() {
		errChan <- client.Start(port)
	}()

	// Give the goroutine a moment to try and start
	time.Sleep(100 * time.Millisecond)

	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected Start to return an error, but got nil")
		}
		// Check for the specific error message indicating address in use
		if !strings.Contains(err.Error(), "address already in use") && !strings.Contains(err.Error(), "bind: address already in use") {
			t.Errorf("Expected 'address already in use' error, got: %v", err)
		}
	case <-time.After(5 * time.Second): // Timeout if Start doesn't return
		t.Fatal("Start method did not return within 5 seconds")
	}
}

func TestHealthEndpoint(t *testing.T) {
	client := api.NewAPIClient()

	// Use the handler directly for httptest
	testServer := httptest.NewServer(client.Handler())
	defer testServer.Close()

	// Make a GET request to the /api/health endpoint
	resp, err := http.Get(testServer.URL + "/api/health")
	if err != nil {
		t.Fatalf("Failed to make GET request: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Decode the response body
	var responseBody api.MessageResponse
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	// Check the message in the response body
	expectedMessage := "OK"
	if responseBody.Message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, responseBody.Message)
	}
}

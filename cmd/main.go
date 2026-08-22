package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type MessageResponse struct {
	Message string `json:"message"`
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	// Allow access from any origin if needed, or specify the frontend origin.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	response := MessageResponse{Message: "Hello from Go!"}
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Serve static files from the current directory (which will be 'bin' when run from the Makefile)
	fs := http.FileServer(http.Dir("bin"))
	http.Handle("/", fs)

	// API routes
	http.HandleFunc("/api/message", apiHandler)

	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
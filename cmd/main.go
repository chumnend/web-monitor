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
	// Allow access in local development
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Content-Type", "application/json")

	response := MessageResponse{Message: "Hello World"}
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/api/message", apiHandler)

	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

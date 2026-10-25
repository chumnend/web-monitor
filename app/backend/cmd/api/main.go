package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"web-monitor/internal/api"
	"web-monitor/internal/store"
)

func main() {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	pgURL := os.Getenv("PG_URL")
	if pgURL == "" {
		pgURL = os.Getenv("DATABASE_URL")
	}

	secretKey := os.Getenv("SECRET_KEY")
	if secretKey == "" {
		secretKey = os.Getenv("JWT_SECRET")
	}

	var opts []api.Option
	if secretKey != "" {
		opts = append(opts, api.WithSecretKey(secretKey))
	}

	if pgURL != "" {
		db, err := sql.Open("postgres", pgURL)
		if err != nil {
			log.Fatalf("Failed to open database connection: %v", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}

		log.Println("Connected to PostgreSQL database successfully.")
		opts = append(opts, api.WithStore(store.NewPostgresStore(db)))
	} else {
		log.Println("Notice: No PG_URL or DATABASE_URL provided; running with in-memory store.")
	}

	client := api.NewAPIClient(opts...)
	log.Fatal(client.Start(port))
}

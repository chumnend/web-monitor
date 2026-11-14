package main

import (
	"log"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	// Attempt to load .env file. This is for local development.
	// In a container, PG_URL is expected to be set as an environment variable directly.
	err := godotenv.Load(".env")
	if err != nil && !os.IsNotExist(err) {
		// If the error is NOT 'file does not exist', then it's a real loading error
		log.Printf("Warning: Could not load .env file (non-existence is okay): %v", err)
	}

	// PG_URL is expected to be set as an environment variable (either from .env or Dockerfile/system)
	pgURL := os.Getenv("PG_URL")
	if pgURL == "" {
		pgURL = os.Getenv("DATABASE_URL")
	}
	if pgURL == "" {
		log.Fatal("PG_URL or DATABASE_URL not set as environment variable. Please set it in .env or Dockerfile.")
	}

	migrationsPath := "./migrations"

	var m *migrate.Migrate
	deadline := time.Now().Add(30 * time.Second)
	for {
		m, err = migrate.New(
			"file://"+migrationsPath,
			pgURL,
		)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			log.Fatalf("Could not create migrate instance after retrying: %v", err)
		}
		log.Printf("Database not ready for migrations yet (%v); retrying in 1s...", err)
		time.Sleep(1 * time.Second)
	}

	if err = m.Down(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while syncing the database: %v", err)
	}

	log.Println("Database migrations rolled back successfully!")
}

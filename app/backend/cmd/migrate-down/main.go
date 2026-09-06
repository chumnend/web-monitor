package main

import (
	"log"
	"os"

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
		log.Fatal("PG_URL not set as environment variable. Please set it in .env or Dockerfile.")
	}

	migrationsPath := "./migrations"

	m, err := migrate.New(
		"file://"+migrationsPath,
		pgURL,
	)
	if err != nil {
		log.Fatalf("Could not create migrate instance: %v", err)
	}

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while syncing the database: %v", err)
	}

	log.Println("Database migrations applied successfully!")
}

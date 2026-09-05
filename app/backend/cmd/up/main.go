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
	// Load .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	pgURL := os.Getenv("PG_URL")
	if pgURL == "" {
		log.Fatal("PG_URL not set in .env file")
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

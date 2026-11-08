package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"web-monitor/internal/checker"
	"web-monitor/internal/store"
)

func main() {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	pgURL := os.Getenv("PG_URL")
	if pgURL == "" {
		pgURL = os.Getenv("DATABASE_URL")
	}

	var dataStore store.Store
	if pgURL != "" {
		db, err := sql.Open("postgres", pgURL)
		if err != nil {
			log.Fatalf("Failed to open database connection: %v", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}
		log.Println("Worker connected to PostgreSQL successfully.")
		dataStore = store.NewPostgresStore(db)
	} else {
		log.Println("Notice: No PG_URL or DATABASE_URL provided; running worker with in-memory store.")
		dataStore = store.NewMemoryStore()
	}

	engine := checker.NewEngine(dataStore, nil)
	pool := checker.NewWorkerPool(20, 200)
	scheduler := checker.NewScheduler(dataStore, engine, pool, 15*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("Starting site check scheduler...")
	if err := scheduler.Start(ctx); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Listen for shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)

	cancel()
	if ok := scheduler.Stop(15 * time.Second); ok {
		log.Println("Checker worker pool drained and stopped cleanly.")
	} else {
		log.Println("Warning: Shutdown timed out waiting for workers.")
	}
}

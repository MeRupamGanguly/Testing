package main

import (
	"batchscheduling/BatchScheduling/batch"
	"batchscheduling/BatchScheduling/repository"
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
)

func main() {
	connStr := "postgres://postgres:password@localhost:5432/batch_db?sslmode=disable"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer dbConn.Close()

	// Configure DB Connection Pool
	// prevent the app from exhausting Postgres connections under heavy load.
	dbConn.SetMaxOpenConns(25)
	dbConn.SetMaxIdleConns(25)
	dbConn.SetConnMaxLifetime(5 * time.Minute)

	repo := &repository.Repository{Conn: dbConn}
	batch := batch.NewBatch(repo)

	scheduler := cron.New()

	// Every 1 minute, trigger a batch
	_, err = scheduler.AddFunc("*/1 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		batch.RunBatch(ctx, 50, 10)
	})

	if err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	log.Println("Scheduler Online. Waiting for triggers...")
	scheduler.Start()

	// Graceful Shutdown Channel
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit // Block until we receive an OS signal

	log.Println("Shutting down gracefully...")

	// Stop the scheduler from taking new jobs and wait for running jobs to finish
	ctx := scheduler.Stop()
	<-ctx.Done()

	log.Println("Shutdown complete.")
}

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pesokrava/product_reviewer/internal/config"
	"github.com/Pesokrava/product_reviewer/internal/pkg/database"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/Pesokrava/product_reviewer/internal/worker"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger := logger.New(cfg.Env)

	appLogger.Info("Starting rating worker...")

	// Connect to database
	appLogger.Info("Connecting to PostgreSQL...")
	db, err := database.WaitForDB(cfg, 10, 2*time.Second)
	if err != nil {
		appLogger.Fatal("Failed to connect to database", err)
	}
	defer db.Close()

	appLogger.Info("Connected to database")

	// Create rating calculator
	calculator := worker.NewCalculator(db, appLogger)

	// Create rating worker
	ratingWorker := worker.NewRatingWorker(calculator, appLogger)

	// Connect to NATS
	appLogger.Info("Connecting to NATS...")
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		appLogger.Fatal("Failed to connect to NATS", err)
	}
	defer nc.Close()

	appLogger.WithFields(map[string]interface{}{
		"url": cfg.NATS.URL,
	}).Info("Connected to NATS")

	// Subscribe to review events
	subject := "reviews.events"
	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		if err := ratingWorker.HandleEvent(msg.Data); err != nil {
			appLogger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("Failed to handle event", err)
		}
	})
	if err != nil {
		appLogger.Fatal("Failed to subscribe to NATS subject", err)
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			appLogger.Error("Failed to unsubscribe from NATS", err)
		}
	}()

	appLogger.WithFields(map[string]interface{}{
		"subject": subject,
	}).Info("Subscribed to review events")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	appLogger.Info("Received shutdown signal")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ratingWorker.Shutdown(shutdownCtx); err != nil {
		appLogger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Error during shutdown", err)
	}

	appLogger.Info("Rating worker stopped")
}

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pesokrava/product_reviewer/internal/config"
	"github.com/Pesokrava/product_reviewer/internal/delivery/events"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger := logger.New(cfg.Env)
	appLogger.Info("Starting notifier service...")

	// Create NATS consumer
	consumer, err := events.NewConsumer(cfg, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to create NATS consumer", err)
	}
	defer consumer.Close()

	// Subscribe to review events
	if err := consumer.Subscribe("reviews.events", events.LoggingHandler(appLogger)); err != nil {
		appLogger.Fatal("Failed to subscribe to reviews.events", err)
	}

	appLogger.Info("Notifier service started and listening for events...")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down notifier service...")
}

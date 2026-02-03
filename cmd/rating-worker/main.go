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

	// Connect to NATS JetStream
	appLogger.Info("Connecting to NATS JetStream...")
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		appLogger.Fatal("Failed to connect to NATS", err)
	}
	defer nc.Close()

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		appLogger.Fatal("Failed to create JetStream context", err)
	}

	appLogger.WithFields(map[string]any{
		"url": cfg.NATS.URL,
	}).Info("Connected to NATS JetStream")

	// Initialize stream and consumer
	appLogger.Info("Initializing JetStream stream and consumer...")
	streamConfig := worker.NewStreamConfig(js, appLogger)

	if err := streamConfig.EnsureStream(); err != nil {
		appLogger.Fatal("Failed to ensure stream", err)
	}

	if err := streamConfig.EnsureConsumer(); err != nil {
		appLogger.Fatal("Failed to ensure consumer", err)
	}

	// Subscribe to review events using durable consumer
	// JetStream ensures exactly-once delivery with ack tracking
	sub, err := js.PullSubscribe("reviews.events", "rating-worker", nats.ManualAck())
	if err != nil {
		appLogger.Fatal("Failed to subscribe to JetStream consumer", err)
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			appLogger.Error("Failed to unsubscribe from JetStream", err)
		}
	}()

	appLogger.WithFields(map[string]any{
		"stream":   "REVIEWS",
		"consumer": "rating-worker",
	}).Info("Subscribed to JetStream consumer")

	// Process messages in a goroutine
	go func() {
		for {
			// Fetch messages in batches (up to 10 at a time)
			msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					// No messages available, continue polling
					continue
				}
				appLogger.WithFields(map[string]any{
					"error": err.Error(),
				}).Error("Failed to fetch messages from JetStream", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, msg := range msgs {
				// Process the message
				if err := ratingWorker.HandleEvent(msg.Data); err != nil {
					appLogger.WithFields(map[string]any{
						"error": err.Error(),
					}).Error("Failed to handle event", err)

					// Negative acknowledgment - message will be redelivered with exponential backoff
					// After 3 failed attempts (MaxDeliver), message is discarded
					// This is acceptable: next review event will trigger full recalculation
					if nackErr := msg.Nak(); nackErr != nil {
						appLogger.WithFields(map[string]any{
							"error": nackErr.Error(),
						}).Error("Failed to NACK message", nackErr)
					}
					continue
				}

				// Successful processing - acknowledge the message
				if ackErr := msg.Ack(); ackErr != nil {
					appLogger.WithFields(map[string]any{
						"error": ackErr.Error(),
					}).Error("Failed to ACK message", ackErr)
				}
			}
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	appLogger.Info("Received shutdown signal")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ratingWorker.Shutdown(shutdownCtx); err != nil {
		appLogger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Error during shutdown", err)
	}

	appLogger.Info("Rating worker stopped")
}

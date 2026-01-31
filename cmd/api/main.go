package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pesokrava/product_reviewer/internal/config"
	httpDelivery "github.com/Pesokrava/product_reviewer/internal/delivery/http"
	"github.com/Pesokrava/product_reviewer/internal/delivery/http/handler"
	"github.com/Pesokrava/product_reviewer/internal/delivery/events"
	"github.com/Pesokrava/product_reviewer/internal/pkg/cache"
	"github.com/Pesokrava/product_reviewer/internal/pkg/database"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	cacheRepo "github.com/Pesokrava/product_reviewer/internal/repository/cache"
	"github.com/Pesokrava/product_reviewer/internal/repository/postgres"
	"github.com/Pesokrava/product_reviewer/internal/usecase/product"
	"github.com/Pesokrava/product_reviewer/internal/usecase/review"

	_ "github.com/Pesokrava/product_reviewer/docs"
)

// @title Product Reviews API
// @version 1.0
// @description A production-ready product reviews system with RESTful APIs, caching, and event notifications.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://github.com/Pesokrava/product_reviewer
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @tag.name Products
// @tag.description Product management endpoints

// @tag.name Reviews
// @tag.description Review management endpoints

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger := logger.New(cfg.Env)
	appLogger.Info("Starting Product Reviews API...")

	// Connect to PostgreSQL with retry
	appLogger.Info("Connecting to PostgreSQL...")
	db, err := database.WaitForDB(cfg, 10, 2*time.Second)
	if err != nil {
		appLogger.Fatal("Failed to connect to database", err)
	}
	defer db.Close()
	appLogger.Info("Connected to PostgreSQL successfully")

	// Connect to Redis with retry
	appLogger.Info("Connecting to Redis...")
	redisClient, err := cache.WaitForRedis(cfg, 10, 2*time.Second)
	if err != nil {
		appLogger.Fatal("Failed to connect to Redis", err)
	}
	defer redisClient.Close()
	appLogger.Info("Connected to Redis successfully")

	// Connect to NATS
	appLogger.Info("Connecting to NATS...")
	publisher, err := events.NewPublisher(cfg, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to create NATS publisher", err)
	}
	defer publisher.Close()

	// Initialize repositories
	productRepo := postgres.NewProductRepository(db)
	reviewRepo := postgres.NewReviewRepository(db)
	redisCache := cacheRepo.NewRedisCache(
		redisClient,
		cfg.Cache.ProductRatingTTL,
		cfg.Cache.ReviewsListTTL,
	)

	// Initialize services
	productService := product.NewService(productRepo, appLogger)
	reviewService := review.NewService(reviewRepo, redisCache, publisher, appLogger)

	// Initialize HTTP handlers
	productHandler := handler.NewProductHandler(productService, appLogger)
	reviewHandler := handler.NewReviewHandler(reviewService, appLogger)

	// Setup router
	router := httpDelivery.NewRouter(productHandler, reviewHandler, appLogger)
	httpHandler := router.Setup()

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine
	go func() {
		appLogger.Infof("HTTP server listening on port %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatal("HTTP server failed", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Fatal("Server forced to shutdown", err)
	}

	appLogger.Info("Server stopped gracefully")
}

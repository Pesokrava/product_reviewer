//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Pesokrava/product_reviewer/internal/config"
	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/database"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/Pesokrava/product_reviewer/internal/repository/postgres"
	"github.com/Pesokrava/product_reviewer/internal/worker"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

func TestRatingWorker_EndToEnd(t *testing.T) {
	// Load config
	cfg, err := config.Load()
	require.NoError(t, err)

	// Setup logger
	log := logger.New(cfg.Env)

	// Connect to database
	db, err := database.WaitForDB(cfg, 5, 2*time.Second)
	require.NoError(t, err)
	defer db.Close()

	// Connect to NATS
	nc, err := nats.Connect(cfg.NATS.URL)
	require.NoError(t, err)
	defer nc.Close()

	// Create calculator and worker
	calculator := worker.NewCalculator(db, log)
	ratingWorker := worker.NewRatingWorker(calculator, log)

	// Subscribe to review events
	_, err = nc.Subscribe("reviews.events", func(msg *nats.Msg) {
		_ = ratingWorker.HandleEvent(msg.Data)
	})
	require.NoError(t, err)

	// Create repositories
	productRepo := postgres.NewProductRepository(db)
	reviewRepo := postgres.NewReviewRepository(db)

	ctx := context.Background()

	// Create test product
	product := &domain.Product{
		ID:          uuid.New(),
		Name:        "Test Product for Rating Worker",
		Description: strPtr("Integration test product"),
		Price:       99.99,
	}
	err = productRepo.Create(ctx, product)
	require.NoError(t, err)

	// Cleanup function
	defer func() {
		_ = productRepo.Delete(ctx, product.ID)
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = ratingWorker.Shutdown(shutdownCtx)
	}()

	// Create reviews with different ratings
	ratings := []int{5, 4, 5, 3, 5} // Average should be 4.4
	reviewIDs := make([]uuid.UUID, len(ratings))

	for i, rating := range ratings {
		review := &domain.Review{
			ID:         uuid.New(),
			ProductID:  product.ID,
			FirstName:  "Test",
			LastName:   "User",
			ReviewText: "Test review",
			Rating:     rating,
		}
		err = reviewRepo.Create(ctx, review)
		require.NoError(t, err)
		reviewIDs[i] = review.ID

		// Publish event
		event := worker.ReviewEvent{
			Type:      "review.created",
			ProductID: product.ID,
			Timestamp: time.Now(),
		}
		eventData, _ := json.Marshal(event)
		err = nc.Publish("reviews.events", eventData)
		require.NoError(t, err)
	}

	// Wait for event processing (debounce window + processing time)
	time.Sleep(2 * time.Second)

	// Verify rating was updated
	updatedProduct, err := productRepo.GetByID(ctx, product.ID)
	require.NoError(t, err)

	// Expected: (5 + 4 + 5 + 3 + 5) / 5 = 22 / 5 = 4.4
	assert.InDelta(t, 4.4, updatedProduct.AverageRating, 0.1, "Rating should be approximately 4.4")

	// Cleanup reviews
	for _, reviewID := range reviewIDs {
		_ = reviewRepo.Delete(ctx, reviewID)
	}
}

func TestRatingWorker_Debouncing(t *testing.T) {
	// Load config
	cfg, err := config.Load()
	require.NoError(t, err)

	// Setup logger
	log := logger.New(cfg.Env)

	// Connect to database
	db, err := database.WaitForDB(cfg, 5, 2*time.Second)
	require.NoError(t, err)
	defer db.Close()

	// Connect to NATS
	nc, err := nats.Connect(cfg.NATS.URL)
	require.NoError(t, err)
	defer nc.Close()

	// Create calculator and worker
	calculator := worker.NewCalculator(db, log)
	ratingWorker := worker.NewRatingWorker(calculator, log)

	// Subscribe to review events
	_, err = nc.Subscribe("reviews.events", func(msg *nats.Msg) {
		_ = ratingWorker.HandleEvent(msg.Data)
	})
	require.NoError(t, err)

	// Create repositories
	productRepo := postgres.NewProductRepository(db)
	reviewRepo := postgres.NewReviewRepository(db)

	ctx := context.Background()

	// Create test product
	product := &domain.Product{
		ID:          uuid.New(),
		Name:        "Popular Product",
		Description: strPtr("High traffic product"),
		Price:       49.99,
	}
	err = productRepo.Create(ctx, product)
	require.NoError(t, err)

	// Cleanup function
	defer func() {
		_ = productRepo.Delete(ctx, product.ID)
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = ratingWorker.Shutdown(shutdownCtx)
	}()

	// Create 20 reviews rapidly
	reviewIDs := make([]uuid.UUID, 20)
	for i := 0; i < 20; i++ {
		review := &domain.Review{
			ID:         uuid.New(),
			ProductID:  product.ID,
			FirstName:  "Rapid",
			LastName:   "User",
			ReviewText: "Quick review",
			Rating:     (i % 5) + 1, // Cycle through 1-5
		}
		err = reviewRepo.Create(ctx, review)
		require.NoError(t, err)
		reviewIDs[i] = review.ID

		// Publish event immediately
		event := worker.ReviewEvent{
			Type:      "review.created",
			ProductID: product.ID,
			Timestamp: time.Now(),
		}
		eventData, _ := json.Marshal(event)
		err = nc.Publish("reviews.events", eventData)
		require.NoError(t, err)
	}

	// Check that events are being debounced (should be 1 or very few pending)
	time.Sleep(500 * time.Millisecond)
	pendingCount := ratingWorker.GetPendingCount()
	assert.LessOrEqual(t, pendingCount, 2, "Events should be debounced")

	// Wait for final processing
	time.Sleep(2 * time.Second)

	// Verify final rating is correct
	updatedProduct, err := productRepo.GetByID(ctx, product.ID)
	require.NoError(t, err)

	// Expected: (1+2+3+4+5)*4 / 20 = 60/20 = 3.0
	assert.InDelta(t, 3.0, updatedProduct.AverageRating, 0.1, "Final rating should be approximately 3.0")

	// Cleanup reviews
	for _, reviewID := range reviewIDs {
		_ = reviewRepo.Delete(ctx, reviewID)
	}
}

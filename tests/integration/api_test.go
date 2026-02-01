//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Pesokrava/product_reviewer/internal/config"
	"github.com/Pesokrava/product_reviewer/internal/delivery/events"
	httpDelivery "github.com/Pesokrava/product_reviewer/internal/delivery/http"
	"github.com/Pesokrava/product_reviewer/internal/delivery/http/handler"
	"github.com/Pesokrava/product_reviewer/internal/pkg/cache"
	"github.com/Pesokrava/product_reviewer/internal/pkg/database"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	cacheRepo "github.com/Pesokrava/product_reviewer/internal/repository/cache"
	"github.com/Pesokrava/product_reviewer/internal/repository/postgres"
	"github.com/Pesokrava/product_reviewer/internal/usecase/product"
	"github.com/Pesokrava/product_reviewer/internal/usecase/review"
)

func setupTestServer(t *testing.T) http.Handler {
	// Load config
	cfg, err := config.Load()
	require.NoError(t, err)

	// Setup logger
	log := logger.New(cfg.Env)

	// Connect to database
	db, err := database.WaitForDB(cfg, 5, 2*time.Second)
	require.NoError(t, err)

	// Connect to Redis
	redisClient, err := cache.WaitForRedis(cfg, 5, 2*time.Second)
	require.NoError(t, err)

	// Connect to NATS
	publisher, err := events.NewPublisher(cfg, log)
	require.NoError(t, err)

	// Setup repositories
	productRepo := postgres.NewProductRepository(db)
	reviewRepo := postgres.NewReviewRepository(db)
	redisCache := cacheRepo.NewRedisCache(
		redisClient,
		cfg.Cache.ProductRatingTTL,
		cfg.Cache.ReviewsListTTL,
	)

	// Setup services
	productService := product.NewService(productRepo, log)
	reviewService := review.NewService(reviewRepo, redisCache, publisher, log)

	// Setup handlers
	productHandler := handler.NewProductHandler(productService, log)
	reviewHandler := handler.NewReviewHandler(reviewService, log)

	// Setup router
	router := httpDelivery.NewRouter(productHandler, reviewHandler, cfg, log)
	return router.Setup()
}

func TestProductCreateAndGet(t *testing.T) {
	server := setupTestServer(t)

	// Create product
	productJSON := `{
		"name": "Test Product",
		"description": "Test Description",
		"price": 99.99
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewBufferString(productJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var createResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&createResp)
	require.NoError(t, err)

	assert.True(t, createResp["success"].(bool))
	productData := createResp["data"].(map[string]interface{})
	productID := productData["id"].(string)

	// Get product
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s", productID), nil)
	w = httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var getResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&getResp)
	require.NoError(t, err)

	assert.True(t, getResp["success"].(bool))
	getData := getResp["data"].(map[string]interface{})
	assert.Equal(t, "Test Product", getData["name"])
	assert.Equal(t, 99.99, getData["price"])
}

func TestHealthCheck(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp["status"])
}

func TestReviewCreateAndList(t *testing.T) {
	server := setupTestServer(t)

	// Create a product first
	productJSON := `{
		"name": "Review Test Product",
		"description": "Product for review testing",
		"price": 149.99
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewBufferString(productJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var productResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&productResp)
	require.NoError(t, err)
	productData := productResp["data"].(map[string]interface{})
	productID := productData["id"].(string)

	// Create a review
	reviewJSON := fmt.Sprintf(`{
		"product_id": "%s",
		"first_name": "John",
		"last_name": "Doe",
		"review_text": "Excellent product!",
		"rating": 5
	}`, productID)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBufferString(reviewJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var reviewResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&reviewResp)
	require.NoError(t, err)
	assert.True(t, reviewResp["success"].(bool))
	reviewData := reviewResp["data"].(map[string]interface{})
	reviewID := reviewData["id"].(string)
	assert.Equal(t, "John", reviewData["first_name"])
	assert.Equal(t, float64(5), reviewData["rating"])

	// List reviews for the product
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s/reviews?limit=10&offset=0", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var listResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&listResp)
	require.NoError(t, err)
	assert.True(t, listResp["success"].(bool))
	reviews := listResp["data"].([]interface{})
	assert.GreaterOrEqual(t, len(reviews), 1)

	// Update the review (product_id not required in update)
	updateJSON := `{
		"first_name": "John",
		"last_name": "Doe",
		"review_text": "Updated: Still excellent!",
		"rating": 4
	}`

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/reviews/%s", reviewID), bytes.NewBufferString(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updateResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&updateResp)
	require.NoError(t, err)
	assert.True(t, updateResp["success"].(bool))
	updatedData := updateResp["data"].(map[string]interface{})
	assert.Equal(t, float64(4), updatedData["rating"])
	assert.Equal(t, "Updated: Still excellent!", updatedData["review_text"])

	// Delete the review
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/reviews/%s", reviewID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestProductRatingUpdate(t *testing.T) {
	server := setupTestServer(t)

	// Create a product
	productJSON := `{
		"name": "Rating Test Product",
		"description": "Product for rating calculation testing",
		"price": 199.99
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewBufferString(productJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var productResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&productResp)
	require.NoError(t, err)
	productData := productResp["data"].(map[string]interface{})
	productID := productData["id"].(string)

	// Initial rating should be 0
	assert.Equal(t, float64(0), productData["average_rating"])

	// Create first review with rating 5
	reviewJSON := fmt.Sprintf(`{
		"product_id": "%s",
		"first_name": "Alice",
		"last_name": "Smith",
		"review_text": "Perfect!",
		"rating": 5
	}`, productID)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBufferString(reviewJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	// Get product and verify rating updated to 5
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	err = json.NewDecoder(w.Body).Decode(&productResp)
	require.NoError(t, err)
	productData = productResp["data"].(map[string]interface{})
	assert.Equal(t, float64(5), productData["average_rating"])

	// Create second review with rating 3
	reviewJSON = fmt.Sprintf(`{
		"product_id": "%s",
		"first_name": "Bob",
		"last_name": "Jones",
		"review_text": "Good but not perfect",
		"rating": 3
	}`, productID)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBufferString(reviewJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	// Get product and verify average rating is now 4.0 (5+3)/2
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	err = json.NewDecoder(w.Body).Decode(&productResp)
	require.NoError(t, err)
	productData = productResp["data"].(map[string]interface{})
	assert.Equal(t, float64(4), productData["average_rating"])
}

func TestPaginationCaching(t *testing.T) {
	server := setupTestServer(t)

	// Create a product
	productJSON := `{
		"name": "Pagination Test Product",
		"description": "Product for cache testing",
		"price": 299.99
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewBufferString(productJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var productResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&productResp)
	require.NoError(t, err)
	productData := productResp["data"].(map[string]interface{})
	productID := productData["id"].(string)

	// Create 5 reviews
	for i := 1; i <= 5; i++ {
		reviewJSON := fmt.Sprintf(`{
			"product_id": "%s",
			"first_name": "User%d",
			"last_name": "Test",
			"review_text": "Review %d",
			"rating": %d
		}`, productID, i, i, (i%5)+1)

		req = httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBufferString(reviewJSON))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		server.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
	}

	// First request - should be cache miss
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s/reviews?limit=3&offset=0", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var firstResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&firstResp)
	require.NoError(t, err)
	firstReviews := firstResp["data"].([]interface{})
	assert.Len(t, firstReviews, 3)

	// Second request with same params - should be cache hit
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s/reviews?limit=3&offset=0", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var secondResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&secondResp)
	require.NoError(t, err)
	secondReviews := secondResp["data"].([]interface{})
	assert.Len(t, secondReviews, 3)

	// Different pagination - should be cache miss
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s/reviews?limit=2&offset=2", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var thirdResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&thirdResp)
	require.NoError(t, err)
	thirdReviews := thirdResp["data"].([]interface{})
	assert.Len(t, thirdReviews, 2)
}

func TestNonAlignedPaginationOffset(t *testing.T) {
	server := setupTestServer(t)

	// Create a product
	productJSON := `{
		"name": "Offset Test Product",
		"description": "Product for offset testing",
		"price": 399.99
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewBufferString(productJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var productResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&productResp)
	require.NoError(t, err)
	productData := productResp["data"].(map[string]interface{})
	productID := productData["id"].(string)

	// Create 30 reviews
	for i := 1; i <= 30; i++ {
		reviewJSON := fmt.Sprintf(`{
			"product_id": "%s",
			"first_name": "User%d",
			"last_name": "Test",
			"review_text": "Review number %d",
			"rating": %d
		}`, productID, i, i, (i%5)+1)

		req = httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBufferString(reviewJSON))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		server.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
	}

	// Aligned offset (offset=20, limit=20)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s/reviews?limit=20&offset=20", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var alignedResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&alignedResp)
	require.NoError(t, err)
	alignedReviews := alignedResp["data"].([]interface{})

	// Non-aligned offset (offset=25, limit=20) - this should NOT return the same data
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s/reviews?limit=20&offset=25", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var nonAlignedResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&nonAlignedResp)
	require.NoError(t, err)
	nonAlignedReviews := nonAlignedResp["data"].([]interface{})

	// Verify they return different results
	assert.NotEqual(t, len(alignedReviews), len(nonAlignedReviews), "Non-aligned offset should return different number of reviews")

	// Verify the cache keys are different by checking the results are different
	if len(alignedReviews) > 0 && len(nonAlignedReviews) > 0 {
		alignedFirstReview := alignedReviews[0].(map[string]interface{})
		nonAlignedFirstReview := nonAlignedReviews[0].(map[string]interface{})
		assert.NotEqual(t, alignedFirstReview["id"], nonAlignedFirstReview["id"], "Different offsets should return different reviews")
	}
}

func TestConcurrentReviewCreation(t *testing.T) {
	server := setupTestServer(t)

	// Create a product
	productJSON := `{
		"name": "Concurrency Test Product",
		"description": "Product for concurrency testing",
		"price": 499.99
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewBufferString(productJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var productResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&productResp)
	require.NoError(t, err)
	productData := productResp["data"].(map[string]interface{})
	productID := productData["id"].(string)

	// Create 10 reviews concurrently
	done := make(chan bool, 10)
	errors := make(chan error, 10)

	for i := 1; i <= 10; i++ {
		go func(index int) {
			reviewJSON := fmt.Sprintf(`{
				"product_id": "%s",
				"first_name": "Concurrent%d",
				"last_name": "User",
				"review_text": "Concurrent review %d",
				"rating": %d
			}`, productID, index, index, (index%5)+1)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBufferString(reviewJSON))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				errors <- fmt.Errorf("review %d failed with status %d", index, w.Code)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify all reviews were created
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s/reviews?limit=20&offset=0", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var listResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&listResp)
	require.NoError(t, err)
	reviews := listResp["data"].([]interface{})
	assert.GreaterOrEqual(t, len(reviews), 10, "All concurrent reviews should be created")

	// Verify average rating was calculated correctly
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%s", productID), nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	err = json.NewDecoder(w.Body).Decode(&productResp)
	require.NoError(t, err)
	productData = productResp["data"].(map[string]interface{})
	avgRating := productData["average_rating"].(float64)
	assert.Greater(t, avgRating, float64(0), "Average rating should be calculated from concurrent reviews")
}

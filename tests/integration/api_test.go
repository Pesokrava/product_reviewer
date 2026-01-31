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
	router := httpDelivery.NewRouter(productHandler, reviewHandler, log)
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

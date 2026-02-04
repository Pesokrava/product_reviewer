package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/Pesokrava/product_reviewer/internal/usecase/review"
)

// MockReviewCache is a mock implementation of review.ReviewCache
type MockReviewCache struct {
	mock.Mock
}

func (m *MockReviewCache) GetReviewsList(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*domain.Review, int, error) {
	args := m.Called(ctx, productID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*domain.Review), args.Int(1), args.Error(2)
}

func (m *MockReviewCache) SetReviewsList(ctx context.Context, productID uuid.UUID, limit, offset int, reviews []*domain.Review, total int) error {
	args := m.Called(ctx, productID, limit, offset, reviews, total)
	return args.Error(0)
}

func (m *MockReviewCache) InvalidateAllProductCache(ctx context.Context, productID uuid.UUID) error {
	args := m.Called(ctx, productID)
	return args.Error(0)
}

// MockEventPublisher is a mock implementation of review.EventPublisher
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	args := m.Called(ctx, subject, data)
	return args.Error(0)
}

func TestReviewHandler_Create_Success(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	productID := uuid.New()
	requestBody := CreateReviewRequest{
		ProductID:  productID.String(),
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
		return r.ProductID == productID && r.FirstName == "John" && r.Rating == 5
	})).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(nil)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	handler.Create(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockRepo.AssertExpectations(t)

	var response map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "data")
}

func TestReviewHandler_Create_InvalidJSON(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Invalid request body")
}

func TestReviewHandler_Create_InvalidProductID(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	requestBody := CreateReviewRequest{
		ProductID:  "invalid-uuid",
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Invalid product ID")
}

func TestReviewHandler_Create_ValidationError(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	productID := uuid.New()
	requestBody := CreateReviewRequest{
		ProductID:  productID.String(),
		FirstName:  "", // Invalid: empty first name
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReviewHandler_Create_InvalidRating(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	productID := uuid.New()
	requestBody := CreateReviewRequest{
		ProductID:  productID.String(),
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     6, // Invalid: rating > 5
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReviewHandler_Create_RepositoryError(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	productID := uuid.New()
	requestBody := CreateReviewRequest{
		ProductID:  productID.String(),
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mockRepo.On("Create", mock.Anything, mock.Anything).Return(fmt.Errorf("database error"))

	handler.Create(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestReviewHandler_Update_Success(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	reviewID := uuid.New()
	productID := uuid.New()
	existingReview := &domain.Review{
		ID:         reviewID,
		ProductID:  productID,
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}

	requestBody := UpdateReviewRequest{
		FirstName:  "Jane",
		LastName:   "Smith",
		ReviewText: "Updated review text",
		Rating:     4,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/"+reviewID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", reviewID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(existingReview, nil)
	mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(r *domain.Review) bool {
		return r.ID == reviewID && r.FirstName == "Jane" && r.Rating == 4
	})).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(nil)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	handler.Update(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestReviewHandler_Update_InvalidUUID(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	requestBody := UpdateReviewRequest{
		FirstName:  "Jane",
		LastName:   "Smith",
		ReviewText: "Updated review text",
		Rating:     4,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/invalid-uuid", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReviewHandler_Update_InvalidJSON(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	reviewID := uuid.New()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/"+reviewID.String(), bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", reviewID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReviewHandler_Update_NotFound(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	reviewID := uuid.New()

	requestBody := UpdateReviewRequest{
		FirstName:  "Jane",
		LastName:   "Smith",
		ReviewText: "Updated review text",
		Rating:     4,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/reviews/"+reviewID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", reviewID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(nil, domain.ErrNotFound)

	handler.Update(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestReviewHandler_Delete_Success(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	reviewID := uuid.New()
	productID := uuid.New()
	existingReview := &domain.Review{
		ID:        reviewID,
		ProductID: productID,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/reviews/"+reviewID.String(), nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", reviewID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(existingReview, nil)
	mockRepo.On("Delete", mock.Anything, reviewID).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(nil)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	handler.Delete(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestReviewHandler_Delete_InvalidUUID(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/reviews/invalid-uuid", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Delete(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReviewHandler_Delete_NotFound(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	reviewID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/reviews/"+reviewID.String(), nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", reviewID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(nil, domain.ErrNotFound)

	handler.Delete(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestReviewHandler_GetByProductID_Success(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	productID := uuid.New()
	reviews := []*domain.Review{
		{
			ID:         uuid.New(),
			ProductID:  productID,
			FirstName:  "John",
			LastName:   "Doe",
			ReviewText: "Great product!",
			Rating:     5,
		},
		{
			ID:         uuid.New(),
			ProductID:  productID,
			FirstName:  "Jane",
			LastName:   "Smith",
			ReviewText: "Good quality",
			Rating:     4,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID.String()+"/reviews?limit=20&offset=0", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Cache miss scenario
	mockCache.On("GetReviewsList", mock.Anything, productID, 20, 0).Return(nil, 0, fmt.Errorf("cache miss"))
	mockRepo.On("GetByProductID", mock.Anything, productID, 20, 0).Return(reviews, nil)
	mockRepo.On("CountByProductID", mock.Anything, productID).Return(2, nil)
	mockCache.On("SetReviewsList", mock.Anything, productID, 20, 0, reviews, 2).Return(nil)

	handler.GetByProductID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)

	var response map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "data")
	assert.Contains(t, response, "pagination")
}

func TestReviewHandler_GetByProductID_CacheHit(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	productID := uuid.New()
	reviews := []*domain.Review{
		{
			ID:         uuid.New(),
			ProductID:  productID,
			FirstName:  "John",
			LastName:   "Doe",
			ReviewText: "Great product!",
			Rating:     5,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID.String()+"/reviews?limit=20&offset=0", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Cache hit scenario - count is included in cache
	mockCache.On("GetReviewsList", mock.Anything, productID, 20, 0).Return(reviews, 1, nil)

	handler.GetByProductID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertNotCalled(t, "GetByProductID")
	mockRepo.AssertNotCalled(t, "CountByProductID")
	mockCache.AssertExpectations(t)

	var response map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "data")
	assert.Contains(t, response, "pagination")
}

func TestReviewHandler_GetByProductID_InvalidUUID(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/invalid-uuid/reviews", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetByProductID(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response["error"], "Invalid product ID")
}

func TestReviewHandler_GetByProductID_WithPagination(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	productID := uuid.New()
	reviews := []*domain.Review{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID.String()+"/reviews?limit=10&offset=20", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockCache.On("GetReviewsList", mock.Anything, productID, 10, 20).Return(nil, 0, fmt.Errorf("cache miss"))
	mockRepo.On("GetByProductID", mock.Anything, productID, 10, 20).Return(reviews, nil)
	mockRepo.On("CountByProductID", mock.Anything, productID).Return(100, nil)
	mockCache.On("SetReviewsList", mock.Anything, productID, 10, 20, reviews, 100).Return(nil)

	handler.GetByProductID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)

	var response map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)
	pagination := response["pagination"].(map[string]any)
	assert.Equal(t, float64(10), pagination["limit"])
	assert.Equal(t, float64(20), pagination["offset"])
	assert.Equal(t, float64(100), pagination["total"])
}

func TestReviewHandler_GetByProductID_RepositoryError(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockReviewCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := review.NewService(mockRepo, mockCache, mockPublisher, log)
	handler := NewReviewHandler(service, log)

	productID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID.String()+"/reviews", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockCache.On("GetReviewsList", mock.Anything, productID, 20, 0).Return(nil, 0, fmt.Errorf("cache miss"))
	mockRepo.On("GetByProductID", mock.Anything, productID, 20, 0).Return(nil, fmt.Errorf("database error"))

	handler.GetByProductID(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockRepo.AssertExpectations(t)
}

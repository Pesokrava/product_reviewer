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
	"github.com/Pesokrava/product_reviewer/internal/usecase/product"
)

// MockProductRepository is a mock implementation of domain.ProductRepository
type MockProductRepository struct {
	mock.Mock
}

func (m *MockProductRepository) Create(ctx context.Context, prod *domain.Product) error {
	args := m.Called(ctx, prod)
	return args.Error(0)
}

func (m *MockProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Product), args.Error(1)
}

func (m *MockProductRepository) List(ctx context.Context, limit, offset int) ([]*domain.Product, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Product), args.Error(1)
}

func (m *MockProductRepository) Update(ctx context.Context, prod *domain.Product) error {
	args := m.Called(ctx, prod)
	return args.Error(0)
}

func (m *MockProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProductRepository) DeleteWithReviews(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProductRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

// MockReviewRepository is a mock implementation of domain.ReviewRepository
type MockReviewRepository struct {
	mock.Mock
}

func (m *MockReviewRepository) Create(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockReviewRepository) GetByProductID(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	args := m.Called(ctx, productID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockReviewRepository) Update(ctx context.Context, review *domain.Review) error {
	args := m.Called(ctx, review)
	return args.Error(0)
}

func (m *MockReviewRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockReviewRepository) DeleteByProductID(ctx context.Context, productID uuid.UUID) error {
	args := m.Called(ctx, productID)
	return args.Error(0)
}

func (m *MockReviewRepository) CountByProductID(ctx context.Context, productID uuid.UUID) (int, error) {
	args := m.Called(ctx, productID)
	return args.Int(0), args.Error(1)
}

func TestProductHandler_Create_Success(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	requestBody := CreateProductRequest{
		Name:  "Test Product",
		Price: 99.99,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(p *domain.Product) bool {
		return p.Name == "Test Product" && p.Price == 99.99
	})).Return(nil)

	handler.Create(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockRepo.AssertExpectations(t)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "data")
}

func TestProductHandler_Create_InvalidJSON(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "Invalid request body")
}

func TestProductHandler_Create_ValidationError(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	requestBody := CreateProductRequest{
		Name:  "", // Invalid: empty name
		Price: 99.99,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_Create_RepositoryError(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	requestBody := CreateProductRequest{
		Name:  "Test Product",
		Price: 99.99,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mockRepo.On("Create", mock.Anything, mock.Anything).Return(fmt.Errorf("database error"))

	handler.Create(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestProductHandler_GetByID_Success(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()
	expectedProduct := &domain.Product{
		ID:            productID,
		Name:          "Test Product",
		Price:         99.99,
		AverageRating: 4.5,
		Version:       1,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID.String(), nil)
	w := httptest.NewRecorder()

	// Add chi context with URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockRepo.On("GetByID", mock.Anything, productID).Return(expectedProduct, nil)

	handler.GetByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "data")
}

func TestProductHandler_GetByID_InvalidUUID(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/invalid-uuid", nil)
	w := httptest.NewRecorder()

	// Add chi context with invalid UUID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.GetByID(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "Invalid product ID")
}

func TestProductHandler_GetByID_NotFound(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+productID.String(), nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockRepo.On("GetByID", mock.Anything, productID).Return(nil, domain.ErrNotFound)

	handler.GetByID(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestProductHandler_List_Success(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	products := []*domain.Product{
		{
			ID:            uuid.New(),
			Name:          "Product 1",
			Price:         99.99,
			AverageRating: 4.5,
		},
		{
			ID:            uuid.New(),
			Name:          "Product 2",
			Price:         149.99,
			AverageRating: 4.8,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=20&offset=0", nil)
	w := httptest.NewRecorder()

	mockRepo.On("List", mock.Anything, 20, 0).Return(products, nil)
	mockRepo.On("Count", mock.Anything).Return(2, nil)

	handler.List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "data")
	assert.Contains(t, response, "pagination")
}

func TestProductHandler_List_WithPagination(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	products := []*domain.Product{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=10&offset=20", nil)
	w := httptest.NewRecorder()

	mockRepo.On("List", mock.Anything, 10, 20).Return(products, nil)
	mockRepo.On("Count", mock.Anything).Return(100, nil)

	handler.List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	pagination := response["pagination"].(map[string]any)
	assert.Equal(t, float64(10), pagination["limit"])
	assert.Equal(t, float64(20), pagination["offset"])
	assert.Equal(t, float64(100), pagination["total"])
}

func TestProductHandler_List_RepositoryError(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	w := httptest.NewRecorder()

	mockRepo.On("List", mock.Anything, 20, 0).Return(nil, fmt.Errorf("database error"))

	handler.List(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestProductHandler_Update_Success(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()

	requestBody := UpdateProductRequest{
		Name:    "Updated Name",
		Price:   149.99,
		Version: 1,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+productID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(p *domain.Product) bool {
		return p.ID == productID && p.Name == "Updated Name" && p.Price == 149.99 && p.Version == 1
	})).Return(nil)

	handler.Update(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestProductHandler_Update_InvalidUUID(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	requestBody := UpdateProductRequest{
		Name:  "Updated Name",
		Price: 149.99,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/invalid-uuid", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_Update_InvalidJSON(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+productID.String(), bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_Update_Conflict(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()

	requestBody := UpdateProductRequest{
		Name:    "Updated Name",
		Price:   149.99,
		Version: 1,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+productID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockRepo.On("Update", mock.Anything, mock.Anything).Return(domain.ErrConflict)

	handler.Update(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestProductHandler_Update_MissingVersion(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()

	// Request without version field
	requestBody := map[string]any{
		"name":  "Updated Name",
		"price": 149.99,
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+productID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_Update_InvalidVersion(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()

	requestBody := UpdateProductRequest{
		Name:    "Updated Name",
		Price:   149.99,
		Version: 0, // Invalid: version must be >= 1
	}
	bodyBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+productID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Update(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_Delete_Success(t *testing.T) {
	mockRepo := new(MockProductRepository)
	mockReviewRepo := new(MockReviewRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, mockReviewRepo, log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/"+productID.String(), nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockReviewRepo.On("DeleteByProductID", mock.Anything, productID).Return(nil)
	mockRepo.On("Delete", mock.Anything, productID).Return(nil)

	handler.Delete(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	mockRepo.AssertExpectations(t)
	mockReviewRepo.AssertExpectations(t)
}

func TestProductHandler_Delete_InvalidUUID(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, new(MockReviewRepository), log)
	handler := NewProductHandler(service, log)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/invalid-uuid", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handler.Delete(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_Delete_NotFound(t *testing.T) {
	mockRepo := new(MockProductRepository)
	mockReviewRepo := new(MockReviewRepository)
	log := logger.New("test")
	service := product.NewService(mockRepo, mockReviewRepo, log)
	handler := NewProductHandler(service, log)

	productID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/"+productID.String(), nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", productID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	mockReviewRepo.On("DeleteByProductID", mock.Anything, productID).Return(domain.ErrNotFound)

	handler.Delete(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockReviewRepo.AssertExpectations(t)
}

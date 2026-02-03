package review

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

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

func (m *MockReviewRepository) CountByProductID(ctx context.Context, productID uuid.UUID) (int, error) {
	args := m.Called(ctx, productID)
	return args.Int(0), args.Error(1)
}

// MockRedisCache is a mock implementation of cache.RedisCache
type MockRedisCache struct {
	mock.Mock
}

func (m *MockRedisCache) GetReviewsList(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	args := m.Called(ctx, productID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockRedisCache) SetReviewsList(ctx context.Context, productID uuid.UUID, limit, offset int, reviews []*domain.Review) error {
	args := m.Called(ctx, productID, limit, offset, reviews)
	return args.Error(0)
}

func (m *MockRedisCache) InvalidateAllProductCache(ctx context.Context, productID uuid.UUID) error {
	args := m.Called(ctx, productID)
	return args.Error(0)
}

// MockEventPublisher is a mock implementation of EventPublisher
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	args := m.Called(ctx, subject, data)
	return args.Error(0)
}

func TestService_Create_Success(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

	productID := uuid.New()
	review := &domain.Review{
		ProductID:  productID,
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}

	mockRepo.On("Create", mock.Anything, review).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(nil)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	err := service.Create(context.Background(), review)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestService_Create_InvalidInput(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

	review := &domain.Review{
		ProductID:  uuid.New(),
		FirstName:  "", // Invalid: empty first name
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}

	err := service.Create(context.Background(), review)

	assert.Error(t, err)
	assert.Equal(t, domain.ErrInvalidInput, err)
	mockRepo.AssertNotCalled(t, "Create")
	mockCache.AssertNotCalled(t, "InvalidateAllProductCache")
}

func TestService_Create_CacheInvalidationFailure(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

	productID := uuid.New()
	review := &domain.Review{
		ProductID:  productID,
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}

	mockRepo.On("Create", mock.Anything, review).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(assert.AnError)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	// Cache failure should not prevent operation from succeeding
	err := service.Create(context.Background(), review)

	assert.NoError(t, err, "Operation should succeed even when cache fails")
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestService_GetByID_Success(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

	reviewID := uuid.New()
	expectedReview := &domain.Review{
		ID:         reviewID,
		ProductID:  uuid.New(),
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Great product!",
		Rating:     5,
	}

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(expectedReview, nil)

	review, err := service.GetByID(context.Background(), reviewID)

	assert.NoError(t, err)
	assert.Equal(t, expectedReview, review)
	mockRepo.AssertExpectations(t)
}

func TestService_GetByID_NotFound(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

	reviewID := uuid.New()

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(nil, domain.ErrNotFound)

	review, err := service.GetByID(context.Background(), reviewID)

	assert.Error(t, err)
	assert.Equal(t, domain.ErrNotFound, err)
	assert.Nil(t, review)
	mockRepo.AssertExpectations(t)
}

func TestService_GetByProductID_CacheHit(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

	productID := uuid.New()
	expectedReviews := []*domain.Review{
		{ID: uuid.New(), ProductID: productID, FirstName: "John", LastName: "Doe", Rating: 5},
		{ID: uuid.New(), ProductID: productID, FirstName: "Jane", LastName: "Smith", Rating: 4},
	}
	expectedTotal := 2

	mockCache.On("GetReviewsList", mock.Anything, productID, 20, 0).Return(expectedReviews, nil)
	mockRepo.On("CountByProductID", mock.Anything, productID).Return(expectedTotal, nil)

	reviews, total, err := service.GetByProductID(context.Background(), productID, 20, 0)

	assert.NoError(t, err)
	assert.Equal(t, expectedReviews, reviews)
	assert.Equal(t, expectedTotal, total)
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "GetByProductID")
}

func TestService_GetByProductID_CacheMiss(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

	productID := uuid.New()
	expectedReviews := []*domain.Review{
		{ID: uuid.New(), ProductID: productID, FirstName: "John", LastName: "Doe", Rating: 5},
		{ID: uuid.New(), ProductID: productID, FirstName: "Jane", LastName: "Smith", Rating: 4},
	}
	expectedTotal := 2

	mockCache.On("GetReviewsList", mock.Anything, productID, 20, 0).Return(nil, assert.AnError)
	mockRepo.On("GetByProductID", mock.Anything, productID, 20, 0).Return(expectedReviews, nil)
	mockRepo.On("CountByProductID", mock.Anything, productID).Return(expectedTotal, nil)
	mockCache.On("SetReviewsList", mock.Anything, productID, 20, 0, expectedReviews).Return(nil)

	reviews, total, err := service.GetByProductID(context.Background(), productID, 20, 0)

	assert.NoError(t, err)
	assert.Equal(t, expectedReviews, reviews)
	assert.Equal(t, expectedTotal, total)
	mockCache.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestService_Update_Success(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

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
	updatedReview := &domain.Review{
		ID:         reviewID,
		ProductID:  productID, // ProductID is required for validation
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Updated review text",
		Rating:     4,
	}

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(existingReview, nil)
	mockRepo.On("Update", mock.Anything, updatedReview).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(nil)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	err := service.Update(context.Background(), updatedReview)

	assert.NoError(t, err)
	assert.Equal(t, productID, updatedReview.ProductID)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestService_Delete_Success(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

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

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(existingReview, nil)
	mockRepo.On("Delete", mock.Anything, reviewID).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(nil)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	err := service.Delete(context.Background(), reviewID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestService_Update_CacheInvalidationFailure(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

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
	updatedReview := &domain.Review{
		ID:         reviewID,
		FirstName:  "John",
		LastName:   "Doe",
		ReviewText: "Updated review text",
		Rating:     4,
	}

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(existingReview, nil)
	mockRepo.On("Update", mock.Anything, updatedReview).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(assert.AnError)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	// Cache failure should not prevent operation from succeeding
	err := service.Update(context.Background(), updatedReview)

	assert.NoError(t, err, "Operation should succeed even when cache fails")
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestService_Delete_CacheInvalidationFailure(t *testing.T) {
	mockRepo := new(MockReviewRepository)
	mockCache := new(MockRedisCache)
	mockPublisher := new(MockEventPublisher)
	log := logger.New("test")
	service := NewService(mockRepo, mockCache, mockPublisher, log)

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

	mockRepo.On("GetByID", mock.Anything, reviewID).Return(existingReview, nil)
	mockRepo.On("Delete", mock.Anything, reviewID).Return(nil)
	mockCache.On("InvalidateAllProductCache", mock.Anything, productID).Return(assert.AnError)
	mockPublisher.On("Publish", mock.Anything, "reviews.events", mock.Anything).Return(nil)

	// Cache failure should not prevent operation from succeeding
	err := service.Delete(context.Background(), reviewID)

	assert.NoError(t, err, "Operation should succeed even when cache fails")
	mockRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

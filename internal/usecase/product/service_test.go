package product

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

// MockProductRepository is a mock implementation of domain.ProductRepository
type MockProductRepository struct {
	mock.Mock
}

func (m *MockProductRepository) Create(ctx context.Context, product *domain.Product) error {
	args := m.Called(ctx, product)
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

func (m *MockProductRepository) Update(ctx context.Context, product *domain.Product) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}

func (m *MockProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProductRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func TestService_Create_Success(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := NewService(mockRepo, log)

	product := &domain.Product{
		Name:  "Test Product",
		Price: 99.99,
	}

	mockRepo.On("Create", mock.Anything, product).Return(nil)

	err := service.Create(context.Background(), product)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_Create_InvalidInput(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := NewService(mockRepo, log)

	product := &domain.Product{
		Name:  "", // Invalid: empty name
		Price: 99.99,
	}

	err := service.Create(context.Background(), product)

	assert.Error(t, err)
	assert.Equal(t, domain.ErrInvalidInput, err)
	mockRepo.AssertNotCalled(t, "Create")
}

func TestService_GetByID_Success(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := NewService(mockRepo, log)

	productID := uuid.New()
	expectedProduct := &domain.Product{
		ID:    productID,
		Name:  "Test Product",
		Price: 99.99,
	}

	mockRepo.On("GetByID", mock.Anything, productID).Return(expectedProduct, nil)

	product, err := service.GetByID(context.Background(), productID)

	assert.NoError(t, err)
	assert.Equal(t, expectedProduct, product)
	mockRepo.AssertExpectations(t)
}

func TestService_GetByID_NotFound(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := NewService(mockRepo, log)

	productID := uuid.New()

	mockRepo.On("GetByID", mock.Anything, productID).Return(nil, domain.ErrNotFound)

	product, err := service.GetByID(context.Background(), productID)

	assert.Error(t, err)
	assert.Equal(t, domain.ErrNotFound, err)
	assert.Nil(t, product)
	mockRepo.AssertExpectations(t)
}

func TestService_List_Success(t *testing.T) {
	mockRepo := new(MockProductRepository)
	log := logger.New("test")
	service := NewService(mockRepo, log)

	expectedProducts := []*domain.Product{
		{ID: uuid.New(), Name: "Product 1", Price: 99.99},
		{ID: uuid.New(), Name: "Product 2", Price: 149.99},
	}
	expectedTotal := 2

	mockRepo.On("List", mock.Anything, 20, 0).Return(expectedProducts, nil)
	mockRepo.On("Count", mock.Anything).Return(expectedTotal, nil)

	products, total, err := service.List(context.Background(), 20, 0)

	assert.NoError(t, err)
	assert.Equal(t, expectedProducts, products)
	assert.Equal(t, expectedTotal, total)
	mockRepo.AssertExpectations(t)
}

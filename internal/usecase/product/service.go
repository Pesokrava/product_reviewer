package product

import (
	"context"
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	pkgValidator "github.com/Pesokrava/product_reviewer/internal/pkg/validator"
)

// Service handles product business logic
type Service struct {
	repo       domain.ProductRepository
	reviewRepo domain.ReviewRepository
	validate   *validator.Validate
	logger     *logger.Logger
}

// NewService creates a new product service
func NewService(repo domain.ProductRepository, reviewRepo domain.ReviewRepository, log *logger.Logger) *Service {
	return &Service{
		repo:       repo,
		reviewRepo: reviewRepo,
		validate:   pkgValidator.Get(),
		logger:     log,
	}
}

// Create creates a new product
func (s *Service) Create(ctx context.Context, product *domain.Product) error {
	if err := s.validate.Struct(product); err != nil {
		s.logger.Error("Product validation failed", err)
		return domain.ErrInvalidInput
	}

	if err := s.repo.Create(ctx, product); err != nil {
		s.logger.Error("Failed to create product", err)
		return err
	}

	s.logger.WithFields(map[string]any{
		"product_id": product.ID,
		"name":       product.Name,
	}).Info("Product created successfully")

	return nil
}

// GetByID retrieves a product by ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	product, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			s.logger.Debugf("Product not found: %s", id)
		} else {
			s.logger.Error("Failed to get product", err)
		}
		return nil, err
	}

	return product, nil
}

// List retrieves a paginated list of products
func (s *Service) List(ctx context.Context, limit, offset int) ([]*domain.Product, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	products, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Failed to list products", err)
		return nil, 0, err
	}

	total, err := s.repo.Count(ctx)
	if err != nil {
		s.logger.Error("Failed to count products", err)
		return nil, 0, err
	}

	return products, total, nil
}

// Update updates an existing product
func (s *Service) Update(ctx context.Context, product *domain.Product) error {
	if err := s.validate.Struct(product); err != nil {
		s.logger.Error("Product validation failed", err)
		return domain.ErrInvalidInput
	}

	if err := s.repo.Update(ctx, product); err != nil {
		s.logger.Error("Failed to update product", err)
		return err
	}

	s.logger.WithFields(map[string]any{
		"product_id": product.ID,
		"name":       product.Name,
	}).Info("Product updated successfully")

	return nil
}

// Delete soft-deletes a product and cascades to all its reviews
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteWithReviews(ctx, id); err != nil {
		s.logger.WithFields(map[string]any{
			"product_id": id,
			"error":      err.Error(),
		}).Error("Failed to delete product and reviews", err)
		return err
	}

	s.logger.WithFields(map[string]any{
		"product_id": id,
	}).Info("Product and reviews deleted successfully")

	return nil
}

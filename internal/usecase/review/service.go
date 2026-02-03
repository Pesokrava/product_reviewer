package review

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	pkgValidator "github.com/Pesokrava/product_reviewer/internal/pkg/validator"
)

// EventPublisher defines the interface for publishing events
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// ReviewCache defines the interface for review caching operations
type ReviewCache interface {
	GetReviewsList(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*domain.Review, error)
	SetReviewsList(ctx context.Context, productID uuid.UUID, limit, offset int, reviews []*domain.Review) error
	InvalidateAllProductCache(ctx context.Context, productID uuid.UUID) error
}

// ReviewEvent represents an event related to a review
type ReviewEvent struct {
	EventType string         `json:"event_type"`
	Timestamp time.Time      `json:"timestamp"`
	ProductID uuid.UUID      `json:"product_id"`
	Review    *domain.Review `json:"review"`
}

// Service handles review business logic with caching and event publishing
type Service struct {
	repo      domain.ReviewRepository
	cache     ReviewCache
	publisher EventPublisher
	validate  *validator.Validate
	logger    *logger.Logger
}

// NewService creates a new review service
func NewService(
	repo domain.ReviewRepository,
	cache ReviewCache,
	publisher EventPublisher,
	log *logger.Logger,
) *Service {
	return &Service{
		repo:      repo,
		cache:     cache,
		publisher: publisher,
		validate:  pkgValidator.Get(),
		logger:    log,
	}
}

// Create creates a new review
func (s *Service) Create(ctx context.Context, review *domain.Review) error {
	if err := s.validate.Struct(review); err != nil {
		s.logger.Error("Review validation failed", err)
		return domain.ErrInvalidInput
	}

	if err := s.repo.Create(ctx, review); err != nil {
		s.logger.Error("Failed to create review", err)
		return err
	}

	// Invalidate cache to prevent stale data
	// Non-fatal: if cache is down, accept temporary staleness over API unavailability
	if err := s.cache.InvalidateAllProductCache(ctx, review.ProductID); err != nil {
		s.logger.WithFields(map[string]interface{}{
			"product_id": review.ProductID,
			"error":      err.Error(),
		}).Warn("Failed to invalidate cache, may serve stale data temporarily")
	}

	s.publishEvent(ctx, "review.created", review)

	s.logger.WithFields(map[string]interface{}{
		"review_id":  review.ID,
		"product_id": review.ProductID,
		"rating":     review.Rating,
	}).Info("Review created successfully")

	return nil
}

// GetByID retrieves a review by ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	review, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == domain.ErrNotFound {
			s.logger.Debugf("Review not found: %s", id)
		} else {
			s.logger.Error("Failed to get review", err)
		}
		return nil, err
	}

	return review, nil
}

// GetByProductID retrieves reviews for a product with caching
func (s *Service) GetByProductID(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*domain.Review, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	reviews, err := s.cache.GetReviewsList(ctx, productID, limit, offset)
	if err == nil {
		s.logger.Debugf("Cache hit for product %s reviews (limit=%d, offset=%d)", productID, limit, offset)
		total, err := s.repo.CountByProductID(ctx, productID)
		if err != nil {
			s.logger.Error("Failed to count reviews", err)
			return nil, 0, err
		}
		return reviews, total, nil
	}

	s.logger.Debugf("Cache miss for product %s reviews (limit=%d, offset=%d)", productID, limit, offset)
	reviews, err = s.repo.GetByProductID(ctx, productID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get reviews by product ID", err)
		return nil, 0, err
	}

	total, err := s.repo.CountByProductID(ctx, productID)
	if err != nil {
		s.logger.Error("Failed to count reviews", err)
		return nil, 0, err
	}

	if err := s.cache.SetReviewsList(ctx, productID, limit, offset, reviews); err != nil {
		s.logger.Warnf("Failed to cache reviews for product %s (limit=%d, offset=%d): %v", productID, limit, offset, err)
	}

	return reviews, total, nil
}

// Update updates an existing review
func (s *Service) Update(ctx context.Context, review *domain.Review) error {
	// Product ID is needed for validation, cache invalidation, and events but not provided in update request
	existingReview, err := s.repo.GetByID(ctx, review.ID)
	if err != nil {
		s.logger.Error("Failed to get existing review", err)
		return err
	}

	// Set product ID from existing review before validation
	review.ProductID = existingReview.ProductID

	if err := s.validate.Struct(review); err != nil {
		s.logger.Error("Review validation failed", err)
		return domain.ErrInvalidInput
	}

	if err := s.repo.Update(ctx, review); err != nil {
		s.logger.Error("Failed to update review", err)
		return err
	}

	// Invalidate cache to prevent stale data
	// Non-fatal: if cache is down, accept temporary staleness over API unavailability
	if err := s.cache.InvalidateAllProductCache(ctx, review.ProductID); err != nil {
		s.logger.WithFields(map[string]interface{}{
			"product_id": review.ProductID,
			"error":      err.Error(),
		}).Warn("Failed to invalidate cache, may serve stale data temporarily")
	}

	s.publishEvent(ctx, "review.updated", review)

	s.logger.WithFields(map[string]interface{}{
		"review_id":  review.ID,
		"product_id": review.ProductID,
		"rating":     review.Rating,
	}).Info("Review updated successfully")

	return nil
}

// Delete soft-deletes a review
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Product ID is needed for cache invalidation but only stored in review record
	review, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get review for deletion", err)
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete review", err)
		return err
	}

	// Invalidate cache to prevent stale data
	// Non-fatal: if cache is down, accept temporary staleness over API unavailability
	if err := s.cache.InvalidateAllProductCache(ctx, review.ProductID); err != nil {
		s.logger.WithFields(map[string]interface{}{
			"product_id": review.ProductID,
			"error":      err.Error(),
		}).Warn("Failed to invalidate cache, may serve stale data temporarily")
	}

	s.publishEvent(ctx, "review.deleted", review)

	s.logger.WithFields(map[string]interface{}{
		"review_id":  id,
		"product_id": review.ProductID,
	}).Info("Review deleted successfully")

	return nil
}

// publishEvent publishes a review event (non-blocking)
func (s *Service) publishEvent(ctx context.Context, eventType string, review *domain.Review) {
	event := ReviewEvent{
		EventType: eventType,
		Timestamp: time.Now(),
		ProductID: review.ProductID,
		Review:    review,
	}

	data, err := json.Marshal(event)
	if err != nil {
		s.logger.Errorf(err, "Failed to marshal event for review %s", review.ID)
		return
	}

	// Publish in background to avoid blocking the HTTP response
	// Use detached context with timeout to prevent cancellation when HTTP request completes
	go func() {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.publisher.Publish(publishCtx, "reviews.events", data); err != nil {
			s.logger.Errorf(err, "Failed to publish event for review %s", review.ID)
		}
	}()
}

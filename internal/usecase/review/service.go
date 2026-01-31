package review

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/Pesokrava/product_reviewer/internal/repository/cache"
)

// EventPublisher defines the interface for publishing events
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
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
	cache     *cache.RedisCache
	publisher EventPublisher
	validate  *validator.Validate
	logger    *logger.Logger
	mu        sync.RWMutex
}

// NewService creates a new review service
func NewService(
	repo domain.ReviewRepository,
	cache *cache.RedisCache,
	publisher EventPublisher,
	log *logger.Logger,
) *Service {
	return &Service{
		repo:      repo,
		cache:     cache,
		publisher: publisher,
		validate:  validator.New(),
		logger:    log,
	}
}

// Create creates a new review
func (s *Service) Create(ctx context.Context, review *domain.Review) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validate.Struct(review); err != nil {
		s.logger.Error("Review validation failed", err)
		return domain.ErrInvalidInput
	}

	if err := s.repo.Create(ctx, review); err != nil {
		s.logger.Error("Failed to create review", err)
		return err
	}

	// Stale cache would show incorrect ratings and review lists
	if err := s.cache.InvalidateAllProductCache(ctx, review.ProductID); err != nil {
		s.logger.Warnf("Failed to invalidate cache for product %s: %v", review.ProductID, err)
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
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validate.Struct(review); err != nil {
		s.logger.Error("Review validation failed", err)
		return domain.ErrInvalidInput
	}

	// Product ID is needed for cache invalidation but not provided in update request
	existingReview, err := s.repo.GetByID(ctx, review.ID)
	if err != nil {
		s.logger.Error("Failed to get existing review", err)
		return err
	}

	if err := s.repo.Update(ctx, review); err != nil {
		s.logger.Error("Failed to update review", err)
		return err
	}

	// Preserve product ID from existing review for event and cache operations
	review.ProductID = existingReview.ProductID

	if err := s.cache.InvalidateAllProductCache(ctx, review.ProductID); err != nil {
		s.logger.Warnf("Failed to invalidate cache for product %s: %v", review.ProductID, err)
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
	s.mu.Lock()
	defer s.mu.Unlock()

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

	if err := s.cache.InvalidateAllProductCache(ctx, review.ProductID); err != nil {
		s.logger.Warnf("Failed to invalidate cache for product %s: %v", review.ProductID, err)
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

	// Publish in background to avoid blocking
	go func() {
		if err := s.publisher.Publish(context.Background(), "reviews.events", data); err != nil {
			s.logger.Errorf(err, "Failed to publish event for review %s", review.ID)
		}
	}()
}

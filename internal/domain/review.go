package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Review represents a product review in the system
type Review struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	ProductID  uuid.UUID  `json:"product_id" db:"product_id" validate:"required"`
	FirstName  string     `json:"first_name" db:"first_name" validate:"required,min=1,max=100"`
	LastName   string     `json:"last_name" db:"last_name" validate:"required,min=1,max=100"`
	ReviewText string     `json:"review_text" db:"review_text" validate:"required,min=1,max=5000"`
	Rating     int        `json:"rating" db:"rating" validate:"required,min=1,max=5"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// ReviewRepository defines the interface for review data access
type ReviewRepository interface {
	// Create creates a new review
	Create(ctx context.Context, review *Review) error

	// GetByID retrieves a review by ID (excludes soft-deleted)
	GetByID(ctx context.Context, id uuid.UUID) (*Review, error)

	// GetByProductID retrieves reviews for a product with pagination (excludes soft-deleted)
	GetByProductID(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*Review, error)

	// Update updates an existing review
	Update(ctx context.Context, review *Review) error

	// Delete soft-deletes a review
	Delete(ctx context.Context, id uuid.UUID) error

	// DeleteByProductID soft-deletes all reviews for a product (cascade delete)
	DeleteByProductID(ctx context.Context, productID uuid.UUID) error

	// CountByProductID returns the total number of reviews for a product (excludes soft-deleted)
	CountByProductID(ctx context.Context, productID uuid.UUID) (int, error)
}

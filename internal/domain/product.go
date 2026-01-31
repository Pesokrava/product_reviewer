package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Product represents a product in the system
type Product struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	Name          string     `json:"name" db:"name" validate:"required,min=1,max=255"`
	Description   *string    `json:"description,omitempty" db:"description"`
	Price         float64    `json:"price" db:"price" validate:"required,gte=0"`
	AverageRating float64    `json:"average_rating" db:"average_rating"`
	Version       int        `json:"version" db:"version"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// ProductRepository defines the interface for product data access
type ProductRepository interface {
	// Create creates a new product
	Create(ctx context.Context, product *Product) error

	// GetByID retrieves a product by ID (excludes soft-deleted)
	GetByID(ctx context.Context, id uuid.UUID) (*Product, error)

	// List retrieves a paginated list of products (excludes soft-deleted)
	List(ctx context.Context, limit, offset int) ([]*Product, error)

	// Update updates an existing product
	Update(ctx context.Context, product *Product) error

	// Delete soft-deletes a product
	Delete(ctx context.Context, id uuid.UUID) error

	// Count returns the total number of products (excludes soft-deleted)
	Count(ctx context.Context) (int, error)
}

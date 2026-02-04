package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/Pesokrava/product_reviewer/internal/domain"
)

// ReviewRepository implements domain.ReviewRepository for PostgreSQL
type ReviewRepository struct {
	db *sqlx.DB
}

// NewReviewRepository creates a new PostgreSQL review repository
func NewReviewRepository(db *sqlx.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

// Create creates a new review
func (r *ReviewRepository) Create(ctx context.Context, review *domain.Review) error {
	// Return domain.ErrNotFound instead of cryptic foreign key constraint violation
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND deleted_at IS NULL)`
	err := r.db.GetContext(ctx, &exists, checkQuery, review.ProductID)
	if err != nil {
		return err
	}
	if !exists {
		return domain.ErrNotFound
	}

	query := `
		INSERT INTO reviews (product_id, first_name, last_name, review_text, rating)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	err = r.db.QueryRowxContext(
		ctx,
		query,
		review.ProductID,
		review.FirstName,
		review.LastName,
		review.ReviewText,
		review.Rating,
	).Scan(
		&review.ID,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

// GetByID retrieves a review by ID
func (r *ReviewRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Review, error) {
	query := `
		SELECT id, product_id, first_name, last_name, review_text, rating, created_at, updated_at, deleted_at
		FROM reviews
		WHERE id = $1 AND deleted_at IS NULL
	`

	var review domain.Review
	err := r.db.GetContext(ctx, &review, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	return &review, nil
}

// GetByProductID retrieves reviews for a product with pagination
func (r *ReviewRepository) GetByProductID(ctx context.Context, productID uuid.UUID, limit, offset int) ([]*domain.Review, error) {
	query := `
		SELECT id, product_id, first_name, last_name, review_text, rating, created_at, updated_at, deleted_at
		FROM reviews
		WHERE product_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	var reviews []*domain.Review
	err := r.db.SelectContext(ctx, &reviews, query, productID, limit, offset)
	if err != nil {
		return nil, err
	}

	return reviews, nil
}

// Update updates an existing review
func (r *ReviewRepository) Update(ctx context.Context, review *domain.Review) error {
	query := `
		UPDATE reviews
		SET first_name = $1, last_name = $2, review_text = $3, rating = $4, updated_at = $5
		WHERE id = $6 AND deleted_at IS NULL
		RETURNING updated_at
	`

	review.UpdatedAt = time.Now()

	err := r.db.QueryRowxContext(
		ctx,
		query,
		review.FirstName,
		review.LastName,
		review.ReviewText,
		review.Rating,
		review.UpdatedAt,
		review.ID,
	).Scan(&review.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}

	return nil
}

// Delete soft-deletes a review
func (r *ReviewRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE reviews
		SET deleted_at = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// DeleteByProductID soft-deletes all reviews for a product (cascade delete)
func (r *ReviewRepository) DeleteByProductID(ctx context.Context, productID uuid.UUID) error {
	query := `
		UPDATE reviews
		SET deleted_at = $1
		WHERE product_id = $2 AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, time.Now(), productID)
	if err != nil {
		return err
	}

	return nil
}

// CountByProductID returns the total number of reviews for a product
func (r *ReviewRepository) CountByProductID(ctx context.Context, productID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM reviews WHERE product_id = $1 AND deleted_at IS NULL`

	var count int
	err := r.db.GetContext(ctx, &count, query, productID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

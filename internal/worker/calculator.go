package worker

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Calculator handles rating calculation and database updates
type Calculator struct {
	db     *sqlx.DB
	logger *logger.Logger
}

// NewCalculator creates a new rating calculator
func NewCalculator(db *sqlx.DB, logger *logger.Logger) *Calculator {
	return &Calculator{
		db:     db,
		logger: logger,
	}
}

// CalculateAndUpdate recalculates average rating for a product and updates the database
// Uses full recalculation approach for simplicity and self-correction
func (c *Calculator) CalculateAndUpdate(ctx context.Context, productID uuid.UUID) error {
	query := `
		UPDATE products
		SET
			average_rating = COALESCE(
				(SELECT ROUND(AVG(rating)::numeric, 1)
				 FROM reviews
				 WHERE product_id = $1 AND deleted_at IS NULL),
				0
			),
			updated_at = $2,
			version = version + 1
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := c.db.ExecContext(ctx, query, productID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update product rating: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// Product not found or deleted - not an error, just log
	if rowsAffected == 0 {
		c.logger.WithFields(map[string]any{
			"product_id": productID.String(),
		}).Info("Product not found or deleted, skipping rating update")
		return nil
	}

	c.logger.WithFields(map[string]any{
		"product_id": productID.String(),
	}).Info("Successfully updated product rating")

	return nil
}

// GetCurrentRating retrieves the current average rating for verification (used in tests)
func (c *Calculator) GetCurrentRating(ctx context.Context, productID uuid.UUID) (float64, error) {
	var rating sql.NullFloat64
	query := `SELECT average_rating FROM products WHERE id = $1 AND deleted_at IS NULL`

	err := c.db.GetContext(ctx, &rating, query, productID)
	if err != nil {
		return 0, fmt.Errorf("failed to get current rating: %w", err)
	}

	if !rating.Valid {
		return 0, nil
	}

	return rating.Float64, nil
}

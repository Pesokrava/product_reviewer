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

// ProductRepository implements domain.ProductRepository for PostgreSQL
type ProductRepository struct {
	db *sqlx.DB
}

// NewProductRepository creates a new PostgreSQL product repository
func NewProductRepository(db *sqlx.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// Create creates a new product
func (r *ProductRepository) Create(ctx context.Context, product *domain.Product) error {
	query := `
		INSERT INTO products (name, description, price, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, average_rating, version, created_at, updated_at
	`

	now := time.Now()
	product.CreatedAt = now
	product.UpdatedAt = now

	err := r.db.QueryRowxContext(
		ctx,
		query,
		product.Name,
		product.Description,
		product.Price,
		product.CreatedAt,
		product.UpdatedAt,
	).Scan(
		&product.ID,
		&product.AverageRating,
		&product.Version,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err != nil {
		return err
	}

	return nil
}

// GetByID retrieves a product by ID
func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	query := `
		SELECT id, name, description, price, average_rating, version, created_at, updated_at, deleted_at
		FROM products
		WHERE id = $1 AND deleted_at IS NULL
	`

	var product domain.Product
	err := r.db.GetContext(ctx, &product, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	return &product, nil
}

// List retrieves a paginated list of products
func (r *ProductRepository) List(ctx context.Context, limit, offset int) ([]*domain.Product, error) {
	query := `
		SELECT id, name, description, price, average_rating, version, created_at, updated_at, deleted_at
		FROM products
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	var products []*domain.Product
	err := r.db.SelectContext(ctx, &products, query, limit, offset)
	if err != nil {
		return nil, err
	}

	return products, nil
}

// Update updates an existing product
func (r *ProductRepository) Update(ctx context.Context, product *domain.Product) error {
	query := `
		UPDATE products
		SET name = $1, description = $2, price = $3, updated_at = $4, version = version + 1
		WHERE id = $5 AND deleted_at IS NULL AND version = $6
		RETURNING version, updated_at
	`

	product.UpdatedAt = time.Now()
	oldVersion := product.Version

	err := r.db.QueryRowxContext(
		ctx,
		query,
		product.Name,
		product.Description,
		product.Price,
		product.UpdatedAt,
		product.ID,
		oldVersion,
	).Scan(&product.Version, &product.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrConflict
		}
		return err
	}

	return nil
}

// Delete soft-deletes a product
func (r *ProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE products
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

// Count returns the total number of products
func (r *ProductRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM products WHERE deleted_at IS NULL`

	var count int
	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

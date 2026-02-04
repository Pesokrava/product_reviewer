package worker

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculator_CalculateAndUpdate_Success(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	log := logger.New("test")
	calculator := NewCalculator(sqlxDB, log)

	productID := uuid.New()
	ctx := context.Background()

	// Expect UPDATE query
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute
	err = calculator.CalculateAndUpdate(ctx, productID)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCalculator_CalculateAndUpdate_ProductNotFound(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	log := logger.New("test")
	calculator := NewCalculator(sqlxDB, log)

	productID := uuid.New()
	ctx := context.Background()

	// Product not found (0 rows affected)
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Execute
	err = calculator.CalculateAndUpdate(ctx, productID)

	// Assert - should not return error for missing product
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCalculator_CalculateAndUpdate_ContextTimeout(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	log := logger.New("test")
	calculator := NewCalculator(sqlxDB, log)

	productID := uuid.New()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Simulate slow query
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillDelayFor(100 * time.Millisecond).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Wait for context to timeout
	time.Sleep(10 * time.Millisecond)

	// Execute
	err = calculator.CalculateAndUpdate(ctx, productID)

	// Assert - should return context timeout error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestCalculator_GetCurrentRating_Success(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	log := logger.New("test")
	calculator := NewCalculator(sqlxDB, log)

	productID := uuid.New()
	expectedRating := 4.5
	ctx := context.Background()

	// Expect SELECT query
	rows := sqlmock.NewRows([]string{"average_rating"}).
		AddRow(expectedRating)
	mock.ExpectQuery("SELECT average_rating FROM products").
		WithArgs(productID).
		WillReturnRows(rows)

	// Execute
	rating, err := calculator.GetCurrentRating(ctx, productID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedRating, rating)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCalculator_GetCurrentRating_NullRating(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	log := logger.New("test")
	calculator := NewCalculator(sqlxDB, log)

	productID := uuid.New()
	ctx := context.Background()

	// Expect SELECT query with NULL rating
	rows := sqlmock.NewRows([]string{"average_rating"}).
		AddRow(nil)
	mock.ExpectQuery("SELECT average_rating FROM products").
		WithArgs(productID).
		WillReturnRows(rows)

	// Execute
	rating, err := calculator.GetCurrentRating(ctx, productID)

	// Assert - should return 0 for NULL rating
	assert.NoError(t, err)
	assert.Equal(t, 0.0, rating)
	assert.NoError(t, mock.ExpectationsWereMet())
}

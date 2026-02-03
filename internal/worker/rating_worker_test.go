package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWorker(t *testing.T) (*RatingWorker, sqlmock.Sqlmock, *sqlx.DB) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	log := logger.New("test")
	calculator := NewCalculator(sqlxDB, log)
	worker := NewRatingWorker(calculator, log)

	return worker, mock, sqlxDB
}

func TestRatingWorker_HandleEvent_Success(t *testing.T) {
	worker, mock, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	productID := uuid.New()
	event := ReviewEvent{
		Type:      "review.created",
		ProductID: productID,
		Timestamp: time.Now(),
	}

	eventData, err := json.Marshal(event)
	require.NoError(t, err)

	// Expect UPDATE query after debounce window
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Handle event
	err = worker.HandleEvent(eventData)
	assert.NoError(t, err)

	// Verify pending update was scheduled
	assert.Equal(t, 1, worker.GetPendingCount())

	// Wait for debounce window + processing time
	time.Sleep(debounceWindow + 100*time.Millisecond)

	// Verify update was processed
	assert.Equal(t, 0, worker.GetPendingCount())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingWorker_HandleEvent_InvalidJSON(t *testing.T) {
	worker, _, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	invalidJSON := []byte(`{invalid json}`)

	err := worker.HandleEvent(invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestRatingWorker_Debouncing_MultipleEvents(t *testing.T) {
	worker, mock, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	productID := uuid.New()

	// Expect only ONE database update despite multiple events
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Send 10 events for the same product within debounce window
	for i := 0; i < 10; i++ {
		event := ReviewEvent{
			Type:      "review.created",
			ProductID: productID,
			Timestamp: time.Now(),
		}
		eventData, _ := json.Marshal(event)
		err := worker.HandleEvent(eventData)
		assert.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // Within debounce window
	}

	// Should still have 1 pending update (debounced)
	assert.Equal(t, 1, worker.GetPendingCount())

	// Wait for debounce window + processing time
	time.Sleep(debounceWindow + 200*time.Millisecond)

	// Verify only one update was executed
	assert.Equal(t, 0, worker.GetPendingCount())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingWorker_EventOrdering_IgnoreStaleEvents(t *testing.T) {
	worker, mock, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	productID := uuid.New()
	now := time.Now()

	// Expect only ONE update (for the newer event)
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Send newer event first
	newerEvent := ReviewEvent{
		Type:      "review.created",
		ProductID: productID,
		Timestamp: now.Add(10 * time.Second),
	}
	newerData, _ := json.Marshal(newerEvent)
	err := worker.HandleEvent(newerData)
	assert.NoError(t, err)

	// Send older event (should be ignored)
	olderEvent := ReviewEvent{
		Type:      "review.created",
		ProductID: productID,
		Timestamp: now,
	}
	olderData, _ := json.Marshal(olderEvent)
	err = worker.HandleEvent(olderData)
	assert.NoError(t, err)

	// Should still have 1 pending update (stale event ignored)
	assert.Equal(t, 1, worker.GetPendingCount())

	// Wait for processing
	time.Sleep(debounceWindow + 200*time.Millisecond)

	// Verify only one update
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingWorker_MultipleProducts(t *testing.T) {
	worker, mock, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	product1 := uuid.New()
	product2 := uuid.New()
	product3 := uuid.New()

	// Expect 3 updates (one per product)
	mock.ExpectExec("UPDATE products").
		WithArgs(product1, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE products").
		WithArgs(product2, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE products").
		WithArgs(product3, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Send events for different products
	for _, productID := range []uuid.UUID{product1, product2, product3} {
		event := ReviewEvent{
			Type:      "review.created",
			ProductID: productID,
			Timestamp: time.Now(),
		}
		eventData, _ := json.Marshal(event)
		err := worker.HandleEvent(eventData)
		assert.NoError(t, err)
	}

	// Should have 3 pending updates
	assert.Equal(t, 3, worker.GetPendingCount())

	// Wait for processing
	time.Sleep(debounceWindow + 300*time.Millisecond)

	// Verify all updates executed
	assert.Equal(t, 0, worker.GetPendingCount())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingWorker_GracefulShutdown(t *testing.T) {
	worker, mock, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	productID := uuid.New()

	// Expect one update to complete
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	event := ReviewEvent{
		Type:      "review.created",
		ProductID: productID,
		Timestamp: time.Now(),
	}
	eventData, _ := json.Marshal(event)
	err := worker.HandleEvent(eventData)
	assert.NoError(t, err)

	// Verify pending update
	assert.Equal(t, 1, worker.GetPendingCount())

	// Wait for processing to start
	time.Sleep(debounceWindow + 50*time.Millisecond)

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = worker.Shutdown(ctx)
	assert.NoError(t, err)

	// Verify clean shutdown
	assert.Equal(t, 0, worker.GetPendingCount())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRatingWorker_ShutdownCancelsPendingUpdates(t *testing.T) {
	worker, _, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	productID := uuid.New()

	// Send event
	event := ReviewEvent{
		Type:      "review.created",
		ProductID: productID,
		Timestamp: time.Now(),
	}
	eventData, _ := json.Marshal(event)
	err := worker.HandleEvent(eventData)
	assert.NoError(t, err)

	// Verify pending update
	assert.Equal(t, 1, worker.GetPendingCount())

	// Shutdown immediately (before processing starts)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = worker.Shutdown(ctx)
	assert.NoError(t, err)

	// Verify pending update was cancelled
	assert.Equal(t, 0, worker.GetPendingCount())
}

func TestRatingWorker_ShutdownTimeout(t *testing.T) {
	worker, mock, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	productID := uuid.New()

	// Simulate slow database update
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillDelayFor(10 * time.Second).
		WillReturnResult(sqlmock.NewResult(0, 1))

	event := ReviewEvent{
		Type:      "review.created",
		ProductID: productID,
		Timestamp: time.Now(),
	}
	eventData, _ := json.Marshal(event)
	err := worker.HandleEvent(eventData)
	assert.NoError(t, err)

	// Wait for processing to start
	time.Sleep(debounceWindow + 50*time.Millisecond)

	// Shutdown with short timeout (should timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = worker.Shutdown(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestRatingWorker_RetryLogic(t *testing.T) {
	worker, mock, sqlxDB := setupTestWorker(t)
	defer sqlxDB.Close()

	productID := uuid.New()

	// Simulate 2 failures then success
	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnError(assert.AnError)

	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnError(assert.AnError)

	mock.ExpectExec("UPDATE products").
		WithArgs(productID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	event := ReviewEvent{
		Type:      "review.created",
		ProductID: productID,
		Timestamp: time.Now(),
	}
	eventData, _ := json.Marshal(event)
	err := worker.HandleEvent(eventData)
	assert.NoError(t, err)

	// Wait for processing with retries (debounce + 3 attempts with backoff)
	time.Sleep(debounceWindow + 1*time.Second)

	// Verify all retries executed
	assert.NoError(t, mock.ExpectationsWereMet())
}

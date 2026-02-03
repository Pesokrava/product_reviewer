package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	// Debounce window - collect events for same product within this duration
	debounceWindow = 1 * time.Second

	// Retry configuration
	maxRetries     = 3
	initialBackoff = 100 * time.Millisecond
)

// ReviewEvent represents a review event from NATS
type ReviewEvent struct {
	Type      string    `json:"type"`
	ProductID uuid.UUID `json:"product_id"`
	Timestamp time.Time `json:"timestamp"`
}

// RatingWorker processes review events and updates product ratings asynchronously
type RatingWorker struct {
	calculator *Calculator
	logger     *logger.Logger

	// Debouncing state
	mu             sync.Mutex
	pendingUpdates map[uuid.UUID]*pendingUpdate
	shutdownCh     chan struct{}
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

type pendingUpdate struct {
	productID uuid.UUID
	timestamp time.Time
	timer     *time.Timer
}

// NewRatingWorker creates a new rating worker
func NewRatingWorker(calculator *Calculator, logger *logger.Logger) *RatingWorker {
	ctx, cancel := context.WithCancel(context.Background())

	return &RatingWorker{
		calculator:     calculator,
		logger:         logger,
		pendingUpdates: make(map[uuid.UUID]*pendingUpdate),
		shutdownCh:     make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// HandleEvent processes a review event
func (w *RatingWorker) HandleEvent(data []byte) error {
	var event ReviewEvent
	if err := json.Unmarshal(data, &event); err != nil {
		w.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to unmarshal review event", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	w.logger.WithFields(map[string]any{
		"type":       event.Type,
		"product_id": event.ProductID.String(),
		"timestamp":  event.Timestamp,
	}).Info("Received review event")

	// Schedule rating update with debouncing
	w.scheduleUpdate(event.ProductID, event.Timestamp)

	return nil
}

// scheduleUpdate implements debouncing logic
// Multiple events for same product within debounce window result in single DB update
func (w *RatingWorker) scheduleUpdate(productID uuid.UUID, timestamp time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if already shutting down
	select {
	case <-w.shutdownCh:
		w.logger.Info("Worker shutting down, ignoring new event")
		return
	default:
	}

	existing, found := w.pendingUpdates[productID]

	// If we have a pending update, check if this event is newer
	if found {
		// Ignore stale events
		if timestamp.Before(existing.timestamp) {
			w.logger.WithFields(map[string]any{
				"product_id":       productID.String(),
				"existing_ts":      existing.timestamp,
				"event_ts":         timestamp,
			}).Debug("Ignoring stale event")
			return
		}

		// Cancel existing timer (we'll create a new one)
		existing.timer.Stop()
		w.logger.WithFields(map[string]any{
			"product_id": productID.String(),
		}).Debug("Debouncing: resetting timer for product")
	} else {
		// New product, increment wait group
		w.wg.Add(1)
	}

	// Create new timer for debounced update
	timer := time.AfterFunc(debounceWindow, func() {
		w.processUpdate(productID)
	})

	w.pendingUpdates[productID] = &pendingUpdate{
		productID: productID,
		timestamp: timestamp,
		timer:     timer,
	}
}

// processUpdate executes the rating calculation with retry logic
func (w *RatingWorker) processUpdate(productID uuid.UUID) {
	defer w.wg.Done()

	w.mu.Lock()
	delete(w.pendingUpdates, productID)
	w.mu.Unlock()

	w.logger.WithFields(map[string]any{
		"product_id": productID.String(),
	}).Info("Processing rating update")

	// Retry loop with exponential backoff
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			w.logger.WithFields(map[string]any{
				"product_id": productID.String(),
				"attempt":    attempt + 1,
				"backoff_ms": backoff.Milliseconds(),
			}).Warn("Retrying rating update")

			select {
			case <-time.After(backoff):
				// Continue with retry
			case <-w.ctx.Done():
				w.logger.Info("Worker context cancelled, aborting retry")
				return
			}

			backoff *= 2
		}

		// Create context with timeout for each attempt
		ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
		err := w.calculator.CalculateAndUpdate(ctx, productID)
		cancel()

		if err == nil {
			return
		}

		lastErr = err
		w.logger.WithFields(map[string]any{
			"product_id": productID.String(),
			"attempt":    attempt + 1,
			"error":      err.Error(),
		}).Error("Failed to update rating", err)
	}

	// All retries exhausted
	w.logger.WithFields(map[string]any{
		"product_id":  productID.String(),
		"max_retries": maxRetries,
		"error":       lastErr.Error(),
	}).Error("Rating update failed after all retries", lastErr)
}

// Shutdown gracefully shuts down the worker
// Cancels pending timers and waits for in-flight updates to complete
func (w *RatingWorker) Shutdown(ctx context.Context) error {
	w.logger.Info("Shutting down rating worker...")

	// Signal shutdown to prevent new updates
	close(w.shutdownCh)

	// Cancel context to stop retries
	w.cancel()

	// Cancel all pending timers
	w.mu.Lock()
	pendingCount := len(w.pendingUpdates)
	for _, update := range w.pendingUpdates {
		update.timer.Stop()
		w.wg.Done() // Decrement counter for cancelled updates
	}
	w.pendingUpdates = make(map[uuid.UUID]*pendingUpdate)
	w.mu.Unlock()

	w.logger.WithFields(map[string]any{
		"cancelled_updates": pendingCount,
	}).Info("Cancelled pending updates")

	// Wait for in-flight updates to complete or context timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("All in-flight updates completed")
		return nil
	case <-ctx.Done():
		w.logger.Warn("Shutdown timeout reached, forcing exit")
		return ctx.Err()
	}
}

// GetPendingCount returns the number of pending updates (used for monitoring/testing)
func (w *RatingWorker) GetPendingCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.pendingUpdates)
}

package events

import (
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

const (
	// StreamName is the JetStream stream for review events
	StreamName = "REVIEWS"

	// StreamSubjects defines the subjects this stream listens to
	StreamSubjects = "reviews.events"

	// ConsumerName is the durable consumer for rating worker
	ConsumerName = "rating-worker"

	// MaxDeliveryAttempts is the max number of delivery attempts before discarding
	// After 3 failed attempts, message is discarded - next review event will recalculate
	MaxDeliveryAttempts = 3

	// AckWait is how long to wait for acknowledgment before redelivery
	AckWait = 30 * time.Second
)

// StreamConfig holds the JetStream stream configuration
type StreamConfig struct {
	js     nats.JetStreamContext
	logger *logger.Logger
}

// NewStreamConfig creates a new stream configuration helper
func NewStreamConfig(js nats.JetStreamContext, log *logger.Logger) *StreamConfig {
	return &StreamConfig{
		js:     js,
		logger: log,
	}
}

// EnsureStream creates or updates the JetStream stream for review events
// Stream configuration:
// - Retention: Work queue (messages deleted after ack or max deliver)
// - Storage: File (survives restarts)
// - Replicas: 1 (single node)
// - MaxAge: 24 hours (stale events are not useful for recalculation)
func (s *StreamConfig) EnsureStream() error {
	stream, err := s.js.StreamInfo(StreamName)

	if errors.Is(err, nats.ErrStreamNotFound) {
		// Create new stream
		s.logger.WithFields(map[string]interface{}{
			"stream":   StreamName,
			"subjects": StreamSubjects,
		}).Info("Creating JetStream stream")

		_, err = s.js.AddStream(&nats.StreamConfig{
			Name:        StreamName,
			Subjects:    []string{StreamSubjects},
			Retention:   nats.WorkQueuePolicy, // Messages deleted after ack
			Storage:     nats.FileStorage,     // Persisted to disk
			Replicas:    1,
			MaxAge:      24 * time.Hour,   // Keep messages for 24 hours max
			Discard:     nats.DiscardOld,  // Discard old messages when limits reached
			Description: "Review events stream for rating calculation",
		})

		if err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}

		s.logger.Info("JetStream stream created successfully")
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get stream info: %w", err)
	}

	// Stream exists
	s.logger.WithFields(map[string]interface{}{
		"stream":   stream.Config.Name,
		"messages": stream.State.Msgs,
		"bytes":    stream.State.Bytes,
	}).Info("JetStream stream already exists")

	return nil
}

// EnsureConsumer creates or updates the durable consumer for the rating worker
// Consumer configuration:
// - Durable: Survives worker restarts
// - AckExplicit: Worker must explicitly acknowledge messages
// - MaxDeliver: 3 attempts then discard (next review event will recalculate)
// - AckWait: 30 seconds to process and ack
// - BackOff: Exponential backoff between retries (1s, 2s, 4s)
//
// Note: Messages that fail after 3 attempts are discarded, not sent to DLQ.
// This is acceptable because rating calculation is idempotent and based on
// database state - the next review event will trigger a full recalculation.
func (s *StreamConfig) EnsureConsumer() error {
	consumerInfo, err := s.js.ConsumerInfo(StreamName, ConsumerName)

	if errors.Is(err, nats.ErrConsumerNotFound) {
		// Create new consumer
		s.logger.WithFields(map[string]interface{}{
			"stream":   StreamName,
			"consumer": ConsumerName,
		}).Info("Creating JetStream consumer")

		_, err = s.js.AddConsumer(StreamName, &nats.ConsumerConfig{
			Durable:       ConsumerName,
			AckPolicy:     nats.AckExplicitPolicy, // Require explicit ack
			AckWait:       AckWait,
			MaxDeliver:    MaxDeliveryAttempts,
			FilterSubject: StreamSubjects,
			// Exponential backoff between redeliveries: 1s, 2s, 4s
			BackOff: []time.Duration{
				1 * time.Second,
				2 * time.Second,
				4 * time.Second,
			},
			Description: "Rating worker consumer for processing review events",
		})

		if err != nil {
			return fmt.Errorf("failed to create consumer: %w", err)
		}

		s.logger.Info("JetStream consumer created successfully")
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get consumer info: %w", err)
	}

	// Consumer exists
	s.logger.WithFields(map[string]interface{}{
		"consumer":    consumerInfo.Name,
		"pending":     consumerInfo.NumPending,
		"redelivered": consumerInfo.NumRedelivered,
		"ack_pending": consumerInfo.NumAckPending,
	}).Info("JetStream consumer already exists")

	return nil
}

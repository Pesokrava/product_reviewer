package events

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/Pesokrava/product_reviewer/internal/config"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

// Publisher handles publishing events to NATS JetStream
type Publisher struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	logger *logger.Logger
}

// NewPublisher creates a new NATS JetStream publisher
func NewPublisher(cfg *config.Config, log *logger.Logger) (*Publisher, error) {
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"url": cfg.NATS.URL,
	}).Info("Connected to NATS JetStream")

	return &Publisher{
		nc:     nc,
		js:     js,
		logger: log,
	}, nil
}

// Publish publishes a message to a NATS JetStream subject
// JetStream ensures message durability and delivery guarantees
func (p *Publisher) Publish(ctx context.Context, subject string, data []byte) error {
	// Publish with acknowledgment - ensures message is stored before returning
	pubAck, err := p.js.Publish(subject, data, nats.Context(ctx))
	if err != nil {
		p.logger.WithFields(map[string]interface{}{
			"subject": subject,
			"error":   err.Error(),
		}).Error("Failed to publish message to JetStream", err)
		return fmt.Errorf("failed to publish to JetStream: %w", err)
	}

	p.logger.WithFields(map[string]interface{}{
		"subject":  subject,
		"stream":   pubAck.Stream,
		"sequence": pubAck.Sequence,
	}).Debug("Published message to JetStream")

	return nil
}

// Close closes the NATS connection
func (p *Publisher) Close() {
	if p.nc != nil {
		p.nc.Close()
		p.logger.Info("NATS publisher connection closed")
	}
}

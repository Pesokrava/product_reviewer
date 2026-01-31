package events

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/Pesokrava/product_reviewer/internal/config"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

// Publisher handles publishing events to NATS
type Publisher struct {
	nc     *nats.Conn
	logger *logger.Logger
}

// NewPublisher creates a new NATS publisher
func NewPublisher(cfg *config.Config, log *logger.Logger) (*Publisher, error) {
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Infof("Connected to NATS at %s", cfg.NATS.URL)

	return &Publisher{
		nc:     nc,
		logger: log,
	}, nil
}

// Publish publishes a message to a NATS subject
func (p *Publisher) Publish(ctx context.Context, subject string, data []byte) error {
	if err := p.nc.Publish(subject, data); err != nil {
		p.logger.Errorf(err, "Failed to publish message to subject %s", subject)
		return err
	}

	p.logger.Debugf("Published message to subject %s", subject)
	return nil
}

// Close closes the NATS connection
func (p *Publisher) Close() {
	if p.nc != nil {
		p.nc.Close()
		p.logger.Info("NATS publisher connection closed")
	}
}

package events

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/Pesokrava/product_reviewer/internal/config"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

// Consumer handles consuming events from NATS
type Consumer struct {
	nc     *nats.Conn
	logger *logger.Logger
	sub    *nats.Subscription
}

// NewConsumer creates a new NATS consumer
func NewConsumer(cfg *config.Config, log *logger.Logger) (*Consumer, error) {
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Infof("Connected to NATS at %s", cfg.NATS.URL)

	return &Consumer{
		nc:     nc,
		logger: log,
	}, nil
}

// Subscribe subscribes to a NATS subject and processes messages
func (c *Consumer) Subscribe(subject string, handler func(data []byte) error) error {
	sub, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		c.logger.Debugf("Received message on subject %s", subject)

		if err := handler(msg.Data); err != nil {
			c.logger.Errorf(err, "Failed to handle message on subject %s", subject)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to subject %s: %w", subject, err)
	}

	c.sub = sub
	c.logger.Infof("Subscribed to NATS subject: %s", subject)
	return nil
}

// Close closes the NATS connection
func (c *Consumer) Close() {
	if c.sub != nil {
		if err := c.sub.Unsubscribe(); err != nil {
			c.logger.Warnf("Failed to unsubscribe from NATS: %v", err)
		}
	}
	if c.nc != nil {
		c.nc.Close()
		c.logger.Info("NATS consumer connection closed")
	}
}

// LoggingHandler creates a simple handler that logs all events
func LoggingHandler(log *logger.Logger) func(data []byte) error {
	return func(data []byte) error {
		// Pretty print the JSON
		var event map[string]interface{}
		if err := json.Unmarshal(data, &event); err != nil {
			log.Error("Failed to unmarshal event", err)
			return err
		}

		prettyJSON, err := json.MarshalIndent(event, "", "  ")
		if err != nil {
			log.Error("Failed to marshal pretty JSON", err)
			return err
		}

		log.Infof("Received event:\n%s", string(prettyJSON))
		return nil
	}
}

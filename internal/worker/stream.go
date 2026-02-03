package worker

import (
	"github.com/nats-io/nats.go"

	"github.com/Pesokrava/product_reviewer/internal/delivery/events"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

// NewStreamConfig creates a new stream configuration helper
// This is a wrapper around events.NewStreamConfig for convenience
func NewStreamConfig(js nats.JetStreamContext, log *logger.Logger) *events.StreamConfig {
	return events.NewStreamConfig(js, log)
}

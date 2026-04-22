package eventbroker

import (
	"context"
	"mail-service/config"
)

// MessageHandler handles a single broker message in a transport-agnostic form.
type MessageHandler func(ctx context.Context, subject string, payload []byte) error

// EventBroker is the transport-agnostic contract used by mail-service
// presentation adapters.
type EventBroker interface {
	Publish(ctx context.Context, subject string, payload []byte) error
	Subscribe(ctx context.Context, subject string, handler MessageHandler) error
	RunPullConsumer(ctx context.Context, cfg config.PullConsumerConfig, handler MessageHandler) error
	Close()
}

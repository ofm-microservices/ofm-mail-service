package nats

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"mail-service/config"
	mail "mail-service/internal/domain"
	eventbroker "mail-service/internal/presentation/event_broker"
)

// MailCommandSubscriber consumes mail send commands from NATS.
type MailCommandSubscriber interface {
	Subscribe(ctx context.Context) error
}

// MailMessageMapper translates mail command payloads into application requests
// and result events.
type MailMessageMapper interface {
	ToSendRequest(cmd sendMailCommand) (mail.SendRequest, error)
	ToFailureResultPayload(cmd sendMailCommand, reason string) ([]byte, error)
	ToSuccessResultPayload(result *mail.SendResult) ([]byte, error)
}

// PullConsumerConfigValidator validates pull-consumer runtime configuration
// before the broker touches JetStream state.
type PullConsumerConfigValidator interface {
	Validate(cfg config.PullConsumerConfig) error
}

// PullConsumerRuntime represents one configured JetStream pull-consumer
// runtime.
type PullConsumerRuntime interface {
	Start(ctx context.Context)
}

// PullConsumerRuntimeFactory builds the runtime used by the broker after the
// config is validated.
type PullConsumerRuntimeFactory interface {
	Create(
		nc *nats.Conn,
		log logging.Logger,
		cfg config.PullConsumerConfig,
		handler eventbroker.MessageHandler,
	) (PullConsumerRuntime, error)
}

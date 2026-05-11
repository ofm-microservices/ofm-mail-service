package nats

import (
	"context"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/natstrace"
	"mail-service/config"
	eventbroker "mail-service/internal/presentation/event_broker"
	"time"

	"github.com/nats-io/nats.go"
)

type natsBroker struct {
	nc             *nats.Conn
	log            logging.Logger
	validator      PullConsumerConfigValidator
	runtimeFactory PullConsumerRuntimeFactory
}

// NewBroker constructs the concrete NATS event broker used by mail-service.
func NewBroker(cfg config.NATSConfig, log logging.Logger) (eventbroker.EventBroker, error) {
	if cfg.URL == "" {
		return nil, ErrEmptyNATSURL
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	opts := []nats.Option{
		nats.Name("mail-service"),
		nats.MaxReconnects(-1),
	}

	if cfg.User != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, WrapConnectToNATSError(err)
	}

	lg := log.With(
		logging.String("module", "nats-broker"),
		logging.String("nats_url", cfg.URL),
	)
	lg.Info("nats broker connected")

	return &natsBroker{
		nc:             nc,
		log:            lg,
		validator:      newPullConsumerConfigValidator(),
		runtimeFactory: newPullConsumerRuntimeFactory(),
	}, nil
}

func (b *natsBroker) Publish(ctx context.Context, subject string, payload []byte) error {
	b.log.Debug("publishing message", logging.String("subject", subject), logging.Int("bytes", len(payload)))

	if err := b.nc.PublishMsg(natstrace.NewMessage(ctx, subject, payload)); err != nil {
		return WrapPublishToNATSError(subject, err)
	}
	if err := Flush(ctx, b.nc); err != nil {
		return WrapFlushNATSPublisherError(err)
	}

	return nil
}

func (b *natsBroker) Subscribe(ctx context.Context, subject string, handler eventbroker.MessageHandler) error {
	b.log.Info("subscribing to subject", logging.String("subject", subject))

	_, err := b.nc.Subscribe(subject, func(msg *nats.Msg) {
		msgCtx := natstrace.ContextFromMessage(ctx, msg)
		if err := handler(msgCtx, msg.Subject, msg.Data); err != nil {
			b.log.Error("message handler failed", logging.String("subject", msg.Subject), logging.Err(err))
			return
		}

		b.log.Info("message handled", logging.String("subject", msg.Subject))
	})
	if err != nil {
		return WrapSubscribeToNATSError(subject, err)
	}

	if err := Flush(ctx, b.nc); err != nil {
		return WrapFlushNATSPublisherError(err)
	}

	return nil
}

func (b *natsBroker) RunPullConsumer(ctx context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler) error {
	validator := b.validator
	if validator == nil {
		validator = newPullConsumerConfigValidator()
	}
	if err := validator.Validate(cfg); err != nil {
		return err
	}

	runtimeFactory := b.runtimeFactory
	if runtimeFactory == nil {
		runtimeFactory = newPullConsumerRuntimeFactory()
	}
	runtime, err := runtimeFactory.Create(b.nc, b.log, cfg, handler)
	if err != nil {
		return err
	}

	runtime.Start(ctx)
	return nil
}

func (b *natsBroker) Close() {
	if b.nc != nil {
		b.log.Info("closing nats broker")
		b.nc.Close()
	}
}

// Flush blocks until buffered NATS publications are acknowledged or the
// supplied context expires.
func Flush(ctx context.Context, nc *nats.Conn) error {
	flushCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		flushCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	return nc.FlushWithContext(flushCtx)
}

// ResolvePullPlan chooses an adaptive pull-consumer tier based on the current
// backlog size.
func ResolvePullPlan(cfg config.PullConsumerConfig, pending int) (string, int, time.Duration) {
	if !cfg.Adaptive.Enabled {
		return "base", cfg.BatchSize, cfg.MaxWait
	}
	if pending >= cfg.Adaptive.HighPending {
		return "high", cfg.Adaptive.HighBatchSize, cfg.Adaptive.HighMaxWait
	}
	if pending >= cfg.Adaptive.MediumPending {
		return "medium", cfg.Adaptive.MediumBatchSize, cfg.Adaptive.MediumMaxWait
	}
	return "low", cfg.Adaptive.LowBatchSize, cfg.Adaptive.LowMaxWait
}

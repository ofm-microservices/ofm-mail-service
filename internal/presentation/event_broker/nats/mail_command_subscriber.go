package nats

import (
	"context"
	"encoding/json"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"mail-service/config"
	app "mail-service/internal/application"
	mail "mail-service/internal/domain"
	eventbroker "mail-service/internal/presentation/event_broker"
)

type mailCommandSubscriber struct {
	broker   eventbroker.EventBroker
	service  app.MailService
	cfg      config.NATSConfig
	resolver FailureReasonResolver
	mapr     MailMessageMapper
	log      logging.Logger
}

// NewMailCommandSubscriber constructs the mail-service command subscriber.
func NewMailCommandSubscriber(
	broker eventbroker.EventBroker,
	service app.MailService,
	cfg config.NATSConfig,
	resolver FailureReasonResolver,
	log logging.Logger,
) (MailCommandSubscriber, error) {
	if broker == nil {
		return nil, ErrNilBroker
	}
	if service == nil {
		return nil, ErrNilMailService
	}
	if resolver == nil {
		return nil, ErrNilFailureReasonResolver
	}
	if log == nil {
		return nil, ErrNilLogger
	}

	return &mailCommandSubscriber{
		broker:   broker,
		service:  service,
		cfg:      cfg,
		resolver: resolver,
		mapr:     newMailMessageMapper(),
		log:      log.With(logging.String("module", "mail-command-subscriber")),
	}, nil
}

// Subscribe starts the pull consumer for mail send commands.
func (s *mailCommandSubscriber) Subscribe(ctx context.Context) error {
	s.log.Info("registering mail pull consumer",
		logging.String("subject", s.cfg.MailSendSubject),
		logging.Int("batch_size", s.cfg.MailBatchSize),
		logging.Any("max_wait", s.cfg.MailMaxWait),
		logging.Int("workers", s.cfg.MailWorkers),
	)

	if err := s.broker.RunPullConsumer(ctx, s.buildPullConsumerConfig(), s.handleMailSendCommand); err != nil {
		return err
	}

	s.log.Info("mail pull consumer ready")
	return nil
}

func (s *mailCommandSubscriber) buildPullConsumerConfig() config.PullConsumerConfig {
	return config.PullConsumerConfig{
		Stream:     s.cfg.MailCommandsStream,
		Subject:    s.cfg.MailSendSubject,
		Durable:    s.cfg.MailSendDurable,
		BatchSize:  s.cfg.MailBatchSize,
		MaxWait:    s.cfg.MailMaxWait,
		Workers:    s.cfg.MailWorkers,
		QueueSize:  s.cfg.MailQueueSize,
		AckWait:    s.cfg.MailAckWait,
		MaxDeliver: s.cfg.MailMaxDeliver,
		Adaptive:   BuildAdaptiveConfig(s.cfg),
	}
}

func (s *mailCommandSubscriber) handleMailSendCommand(ctx context.Context, _ string, payload []byte) error {
	var cmd sendMailCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return WrapUnmarshalSendMailCommandError(err)
	}

	request, err := s.mapr.ToSendRequest(cmd)
	if err != nil {
		return err
	}

	result, err := s.service.Send(ctx, request)
	if err != nil {
		return s.publishFailureResult(ctx, cmd, err)
	}

	return s.publishSuccessResult(ctx, result)
}

func (s *mailCommandSubscriber) publishFailureResult(ctx context.Context, cmd sendMailCommand, err error) error {
	payload, mapErr := s.mapr.ToFailureResultPayload(cmd, s.resolver.SendMailFailureReason(err))
	if mapErr != nil {
		return mapErr
	}

	return s.broker.Publish(ctx, s.cfg.MailSendResultSubject, payload)
}

func (s *mailCommandSubscriber) publishSuccessResult(ctx context.Context, result *mail.SendResult) error {
	payload, err := s.mapr.ToSuccessResultPayload(result)
	if err != nil {
		return err
	}

	return s.broker.Publish(ctx, s.cfg.MailSendResultSubject, payload)
}

// BuildAdaptiveConfig projects NATS config values into a generic adaptive pull
// consumer config.
func BuildAdaptiveConfig(cfg config.NATSConfig) config.PullAdaptiveConfig {
	return config.PullAdaptiveConfig{
		Enabled:         cfg.MailAdaptiveEnabled,
		CheckInterval:   cfg.MailAdaptiveCheckInterval,
		MediumPending:   cfg.MailAdaptiveMediumPending,
		HighPending:     cfg.MailAdaptiveHighPending,
		LowBatchSize:    cfg.MailAdaptiveLowBatchSize,
		LowMaxWait:      cfg.MailAdaptiveLowMaxWait,
		MediumBatchSize: cfg.MailAdaptiveMediumBatchSize,
		MediumMaxWait:   cfg.MailAdaptiveMediumMaxWait,
		HighBatchSize:   cfg.MailAdaptiveHighBatchSize,
		HighMaxWait:     cfg.MailAdaptiveHighMaxWait,
	}
}

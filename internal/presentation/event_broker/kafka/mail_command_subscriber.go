package kafka

import (
	"context"
	"encoding/json"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"mail-service/config"
	app "mail-service/internal/application"
	eventbroker "mail-service/internal/presentation/event_broker"
)

type mailCommandSubscriber struct {
	broker   eventbroker.EventBroker
	service  app.MailService
	cfg      config.KafkaConfig
	resolver failureReasonResolver
	mapper   mailMessageMapper
	log      logging.Logger
}

// NewMailCommandSubscriber constructs the Kafka mail command consumer.
func NewMailCommandSubscriber(broker eventbroker.EventBroker, service app.MailService, cfg config.KafkaConfig, resolver failureReasonResolver, log logging.Logger) (MailCommandSubscriber, error) {
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
	return &mailCommandSubscriber{broker: broker, service: service, cfg: cfg, resolver: resolver, mapper: newMailMapper(), log: log.With(logging.String("module", "mail-command-subscriber"))}, nil
}

func (s *mailCommandSubscriber) Subscribe(ctx context.Context) error {
	s.log.Info("registering Kafka mail consumer", logging.String("topic", s.cfg.CommandTopic))
	return s.broker.RunPullConsumer(ctx, config.PullConsumerConfig{Subject: s.cfg.CommandTopic}, s.handle)
}

func (s *mailCommandSubscriber) handle(ctx context.Context, _ string, payload []byte) error {
	var cmd sendMailCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return wrapUnmarshal(err)
	}
	request, err := s.mapper.ToSendRequest(cmd)
	if err != nil {
		return err
	}
	result, err := s.service.Send(ctx, request)
	if err != nil {
		encoded, mapErr := s.mapper.ToFailureResultPayload(cmd, s.resolver.SendMailFailureReason(err))
		if mapErr != nil {
			return mapErr
		}
		return s.broker.Publish(ctx, s.cfg.ResultTopic, encoded)
	}
	encoded, err := s.mapper.ToSuccessResultPayload(result)
	if err != nil {
		return err
	}
	return s.broker.Publish(ctx, s.cfg.ResultTopic, encoded)
}

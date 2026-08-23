package appfx

import (
	"context"
	"fmt"
	"github.com/ofm-microservices/ofm-common/pkg/idempotency"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/resilience"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"mail-service/config"
	eventbroker "mail-service/internal/presentation/event_broker"
	broker "mail-service/internal/presentation/event_broker/kafka"
	"time"

	"go.uber.org/fx"
)

// MessagingModule wires the Kafka broker runtime into mail-service.
var MessagingModule = fx.Options(
	fx.Provide(ProvideEventBrokerWithClaims),
)

// InvokeEnsureStream is retained as a compatibility no-op; Kafka topics are
// provisioned by the broker/runtime rather than JetStream bootstrap.
func InvokeEnsureStream(*config.Config, logging.Logger) error { return nil }

// ProvideEventBroker constructs the concrete Kafka event broker.
func ProvideEventBroker(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (eventbroker.EventBroker, error) {
	return provideEventBroker(lc, cfg, lg, nil)
}

// ProvideEventBrokerWithClaims wires Kafka with Redis-backed durable event claims.
func ProvideEventBrokerWithClaims(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger) (eventbroker.EventBroker, error) {
	rdb := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port), Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		return nil, fmt.Errorf("instrument redis claims: %w", err)
	}
	if err := resilience.Retry(context.Background(), resilience.RetryPolicyFromEnv(), func(ctx context.Context, _ int) error { return rdb.Ping(ctx).Err() }); err != nil {
		return nil, fmt.Errorf("connect redis claims: %w", err)
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return rdb.Close() }})
	return provideEventBroker(lc, cfg, lg, &redisEventStore{rdb: rdb})
}

func provideEventBroker(lc fx.Lifecycle, cfg *config.Config, lg logging.Logger, store idempotency.Store) (eventbroker.EventBroker, error) {
	eventBroker, err := broker.NewBroker(cfg.Kafka)
	if err != nil {
		lg.Error("connect kafka failed", logging.Err(err))
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			eventBroker.Close()
			return nil
		},
	})

	return &claimingBroker{EventBroker: eventBroker, store: store}, nil
}

type redisEventStore struct{ rdb *redis.Client }

func (s *redisEventStore) Claim(ctx context.Context, event idempotency.Event) (bool, error) {
	return s.rdb.SetNX(ctx, "mail:processed_events:"+event.EventID, "processing", 15*time.Minute).Result()
}
func (s *redisEventStore) Complete(ctx context.Context, eventID string) error {
	return s.rdb.Set(ctx, "mail:processed_events:"+eventID, "done", 0).Err()
}
func (s *redisEventStore) Release(ctx context.Context, eventID string) error {
	return s.rdb.Del(ctx, "mail:processed_events:"+eventID).Err()
}

type claimingBroker struct {
	eventbroker.EventBroker
	store idempotency.Store
}

func (b *claimingBroker) RunPullConsumer(ctx context.Context, cfg config.PullConsumerConfig, handler eventbroker.MessageHandler) error {
	return b.EventBroker.RunPullConsumer(ctx, cfg, func(ctx context.Context, subject string, payload []byte) error {
		if b.store == nil {
			return handler(ctx, subject, payload)
		}
		event := idempotency.DecodeOrFingerprint(subject, payload)
		claimed, err := b.store.Claim(ctx, event)
		if err != nil || !claimed {
			return err
		}
		if err := handler(ctx, subject, payload); err != nil {
			_ = b.store.Release(ctx, event.EventID)
			return err
		}
		if complete, ok := b.store.(interface {
			Complete(context.Context, string) error
		}); ok {
			return complete.Complete(ctx, event.EventID)
		}
		return nil
	})
}

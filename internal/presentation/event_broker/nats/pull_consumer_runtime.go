package nats

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	"mail-service/config"
	eventbroker "mail-service/internal/presentation/event_broker"
)

type pullConsumerRuntimeFactory struct{}

type pullConsumerRuntime struct {
	sub     *nats.Subscription
	log     logging.Logger
	cfg     config.PullConsumerConfig
	handler eventbroker.MessageHandler
	jobs    chan *nats.Msg
}

func newPullConsumerRuntimeFactory() PullConsumerRuntimeFactory {
	return &pullConsumerRuntimeFactory{}
}

func (f *pullConsumerRuntimeFactory) Create(
	nc *nats.Conn,
	log logging.Logger,
	cfg config.PullConsumerConfig,
	handler eventbroker.MessageHandler,
) (PullConsumerRuntime, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, WrapInitJetStreamContextError(err)
	}

	consumerCfg := &nats.ConsumerConfig{
		Durable:       cfg.Durable,
		FilterSubject: cfg.Subject,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       cfg.AckWait,
		MaxDeliver:    cfg.MaxDeliver,
	}

	if _, err := js.AddConsumer(cfg.Stream, consumerCfg); err != nil {
		if _, updateErr := js.UpdateConsumer(cfg.Stream, consumerCfg); updateErr != nil {
			return nil, WrapEnsureConsumerError(cfg.Stream, cfg.Durable, err, updateErr)
		}
	}

	sub, err := js.PullSubscribe(cfg.Subject, cfg.Durable, nats.BindStream(cfg.Stream), nats.ManualAck())
	if err != nil {
		return nil, WrapCreatePullSubscriberError(cfg.Subject, cfg.Durable, err)
	}

	return &pullConsumerRuntime{
		sub:     sub,
		log:     log,
		cfg:     cfg,
		handler: handler,
		jobs:    make(chan *nats.Msg, cfg.QueueSize),
	}, nil
}

func (r *pullConsumerRuntime) Start(ctx context.Context) {
	var wg sync.WaitGroup

	for i := 0; i < r.cfg.Workers; i++ {
		wg.Add(1)
		workerID := i + 1
		go r.runWorker(ctx, &wg, workerID)
	}

	go r.runFetcher(ctx, &wg)
}

func (r *pullConsumerRuntime) runWorker(ctx context.Context, wg *sync.WaitGroup, workerID int) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-r.jobs:
			if !ok {
				return
			}

			if err := r.handler(ctx, msg.Subject, msg.Data); err != nil {
				r.log.Error("message handler failed",
					logging.String("subject", msg.Subject),
					logging.Int("worker_id", workerID),
					logging.Err(err),
				)
				_ = msg.Nak()
				continue
			}

			if err := msg.Ack(); err != nil {
				r.log.Error("message ack failed",
					logging.String("subject", msg.Subject),
					logging.Int("worker_id", workerID),
					logging.Err(err),
				)
			}
		}
	}
}

func (r *pullConsumerRuntime) runFetcher(ctx context.Context, wg *sync.WaitGroup) {
	defer close(r.jobs)
	defer wg.Wait()

	state := pullConsumerFetchState{
		batch:             r.cfg.BatchSize,
		wait:              r.cfg.MaxWait,
		tier:              "base",
		lastAdaptiveCheck: time.Now(),
	}

	for {
		if r.shouldStop(ctx) {
			return
		}

		r.maybeUpdateAdaptivePlan(&state)

		msgs, err := r.sub.Fetch(state.batch, nats.MaxWait(state.wait))
		if r.handleFetchError(err) {
			continue
		}

		if r.dispatchFetchedMessages(ctx, msgs) {
			return
		}
	}
}

type pullConsumerFetchState struct {
	batch             int
	wait              time.Duration
	tier              string
	lastAdaptiveCheck time.Time
}

func (r *pullConsumerRuntime) shouldStop(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		r.log.Info("pull consumer stopped",
			logging.String("subject", r.cfg.Subject),
			logging.String("durable", r.cfg.Durable),
		)
		return true
	default:
		return false
	}
}

func (r *pullConsumerRuntime) maybeUpdateAdaptivePlan(state *pullConsumerFetchState) {
	if !r.cfg.Adaptive.Enabled || time.Since(state.lastAdaptiveCheck) < r.cfg.Adaptive.CheckInterval {
		return
	}

	state.lastAdaptiveCheck = time.Now()

	info, err := r.sub.ConsumerInfo()
	if err != nil {
		return
	}

	nextTier, nextBatch, nextWait := ResolvePullPlan(r.cfg, int(info.NumPending))
	if nextTier == state.tier && nextBatch == state.batch && nextWait == state.wait {
		return
	}

	state.tier = nextTier
	state.batch = nextBatch
	state.wait = nextWait

	r.log.Info("adaptive pull plan switched",
		logging.String("subject", r.cfg.Subject),
		logging.String("durable", r.cfg.Durable),
		logging.String("tier", state.tier),
		logging.Int("pending", int(info.NumPending)),
		logging.Int("batch_size", state.batch),
		logging.Any("max_wait", state.wait),
	)
}

func (r *pullConsumerRuntime) handleFetchError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	r.log.Error("pull consumer fetch failed",
		logging.String("subject", r.cfg.Subject),
		logging.String("durable", r.cfg.Durable),
		logging.Err(err),
	)
	time.Sleep(250 * time.Millisecond)
	return true
}

func (r *pullConsumerRuntime) dispatchFetchedMessages(ctx context.Context, msgs []*nats.Msg) bool {
	for _, msg := range msgs {
		select {
		case <-ctx.Done():
			return true
		case r.jobs <- msg:
		}
	}

	return false
}

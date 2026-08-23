package appfx

import (
	"context"
	"time"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"mail-service/config"
	app "mail-service/internal/application"
	eventbroker "mail-service/internal/presentation/event_broker"
	events "mail-service/internal/presentation/event_broker/kafka"

	"go.uber.org/fx"
)

// PresentationModule wires message-consumer presentation adapters into the FX
// lifecycle.
var PresentationModule = fx.Options(
	fx.Provide(
		events.NewDomainFailureReasonResolver,
		ProvideMailCommandSubscriber,
	),
	fx.Invoke(InvokeSubscribeMailCommands),
)

// ProvideMailCommandSubscriber constructs the Kafka subscriber that consumes
// mail send commands.
func ProvideMailCommandSubscriber(
	broker eventbroker.EventBroker,
	service app.MailService,
	cfg *config.Config,
	resolver events.FailureReasonResolver,
	lg logging.Logger,
) (events.MailCommandSubscriber, error) {
	return events.NewMailCommandSubscriber(broker, service, cfg.Kafka, resolver, lg)
}

// InvokeSubscribeMailCommands starts background consumers for mail commands.
func InvokeSubscribeMailCommands(
	lc fx.Lifecycle,
	subscriber events.MailCommandSubscriber,
	cfg *config.Config,
	lg logging.Logger,
) {
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runCtx, runCancel := context.WithCancel(context.Background())
			cancel = runCancel

			lg.Info("mail-service initialized", logging.String("env", cfg.App.Env))
			go func() {
				backoff := 100 * time.Millisecond
				for runCtx.Err() == nil {
					err := subscriber.Subscribe(runCtx)
					if runCtx.Err() != nil {
						return
					}
					if err != nil {
						lg.Error("subscribe to mail commands failed; reconnecting", logging.Err(err), logging.String("backoff", backoff.String()))
					} else {
						lg.Warn("mail command consumer stopped; reconnecting", logging.String("backoff", backoff.String()))
					}
					timer := time.NewTimer(backoff)
					select {
					case <-runCtx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
					if backoff < 30*time.Second {
						backoff *= 2
						if backoff > 30*time.Second {
							backoff = 30 * time.Second
						}
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			if cancel != nil {
				cancel()
			}
			return nil
		},
	})
}

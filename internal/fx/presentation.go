package appfx

import (
	"context"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	"mail-service/config"
	app "mail-service/internal/application"
	eventbroker "mail-service/internal/presentation/event_broker"
	events "mail-service/internal/presentation/event_broker/nats"

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

// ProvideMailCommandSubscriber constructs the NATS subscriber that consumes
// mail send commands.
func ProvideMailCommandSubscriber(
	broker eventbroker.EventBroker,
	service app.MailService,
	cfg *config.Config,
	resolver events.FailureReasonResolver,
	lg logging.Logger,
) (events.MailCommandSubscriber, error) {
	return events.NewMailCommandSubscriber(broker, service, cfg.NATS, resolver, lg)
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

			if err := subscriber.Subscribe(runCtx); err != nil {
				lg.Error("subscribe to mail commands failed", logging.Err(err))
				cancel()
				return err
			}

			lg.Info("mail-service initialized", logging.String("env", cfg.App.Env))
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

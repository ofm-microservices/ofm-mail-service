package appfx

import (
	"context"
	"github.com/ofm-microseervices/ofm-common/pkg/logging"
	"mail-service/config"

	"go.uber.org/fx"
)

// LoggerModule provides the shared structured logger for mail-service.
var LoggerModule = fx.Options(
	fx.Provide(ProvideLogger),
)

// ProvideLogger constructs the shared zap-based logger and syncs it on stop.
func ProvideLogger(lc fx.Lifecycle, cfg *config.Config) (logging.Logger, error) {
	lg, err := logging.New("mail-service", cfg.App.Env, cfg.App.LogLevel)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return lg.Sync()
		},
	})

	return lg, nil
}

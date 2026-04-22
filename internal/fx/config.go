package appfx

import (
	"mail-service/config"

	"go.uber.org/fx"
)

// ConfigModule provides configuration loading for mail-service.
var ConfigModule = fx.Options(
	fx.Provide(ProvideConfig),
)

// ProvideConfig loads and validates the mail-service configuration.
func ProvideConfig() (*config.Config, error) {
	return config.Load()
}

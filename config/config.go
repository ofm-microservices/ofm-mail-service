package config

import "github.com/caarlos0/env/v11"

// Config is the root runtime configuration for mail-service.
type Config struct {
	App       AppConfig
	Metrics   MetricsConfig
	Tracing   TracingConfig
	SMTP      SMTPConfig
	NATS      NATSConfig
	Templates TemplatesConfig
}

// Load reads environment variables into Config and applies the declared
// defaults for mail-service.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, WrapParseEnvConfigError(err)
	}

	return cfg, nil
}

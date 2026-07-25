package config

// MetricsConfig defines the Prometheus scrape listener for mail-service.
type MetricsConfig struct {
	Enabled bool   `env:"METRICS_ENABLED" envDefault:"true"`
	Host    string `env:"METRICS_HOST" envDefault:"0.0.0.0"`
	Port    int    `env:"METRICS_PORT" envDefault:"9605"`
	Path    string `env:"METRICS_PATH" envDefault:"/metrics"`
}

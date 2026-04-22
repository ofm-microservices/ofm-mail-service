package config

// AppConfig holds generic process-level runtime settings for mail-service.
type AppConfig struct {
	Env      string `env:"APP_ENV" envDefault:"local"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}

package config

// TracingConfig controls OTLP export for mail-service.
type TracingConfig struct {
	Enabled        bool    `env:"TRACING_ENABLED" envDefault:"true"`
	Endpoint       string  `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"http://127.0.0.1:9099"`
	Protocol       string  `env:"OTEL_EXPORTER_OTLP_PROTOCOL" envDefault:"http/protobuf"`
	SampleRatio    float64 `env:"TRACING_SAMPLE_RATIO" envDefault:"1.0"`
	ServiceVersion string  `env:"SERVICE_VERSION" envDefault:"dev"`
}

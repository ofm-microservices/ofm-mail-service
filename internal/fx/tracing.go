package appfx

import (
	"context"
	"mail-service/config"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	sharedtracing "github.com/ofm-microservices/ofm-common/pkg/observability/tracing"
	"go.uber.org/fx"
)

// TracingModule wires OpenTelemetry tracing for mail-service.
var TracingModule = fx.Options(
	fx.Provide(ProvideTracingConfig),
	fx.Invoke(InvokeInstallTracing),
)

func ProvideTracingConfig(cfg *config.Config) *sharedtracing.Config {
	tcfg := sharedtracing.DefaultConfig("mail-service", cfg.App.Env)
	tcfg.Enabled = cfg.Tracing.Enabled
	tcfg.Endpoint = cfg.Tracing.Endpoint
	tcfg.Protocol = cfg.Tracing.Protocol
	tcfg.SampleRatio = cfg.Tracing.SampleRatio
	tcfg.ServiceVersion = cfg.Tracing.ServiceVersion
	return &tcfg
}

func InvokeInstallTracing(lc fx.Lifecycle, cfg *sharedtracing.Config, lg logging.Logger) {
	if cfg == nil {
		return
	}
	tp, err := sharedtracing.NewTracerProvider(context.Background(), *cfg)
	if err != nil {
		lg.Error("tracing initialization failed", logging.Err(err))
		return
	}
	sharedtracing.SetGlobal(tp)
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return tp.Shutdown(context.Background()) }})
	lg.Info("tracing initialized")
}

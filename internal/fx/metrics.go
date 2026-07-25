package appfx

import (
	"context"
	"mail-service/config"

	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"github.com/ofm-microservices/ofm-common/pkg/observability/metrics"
	"go.uber.org/fx"
)

// MetricsModule wires Prometheus metrics collection and serving.
var MetricsModule = fx.Options(
	fx.Provide(ProvideMetrics),
	fx.Invoke(InvokeRunMetricsServer),
)

// ProvideMetrics constructs and installs the process metrics recorder.
func ProvideMetrics(cfg *config.Config) metrics.Meter {
	if !cfg.Metrics.Enabled {
		meter := metrics.NewNoop()
		metrics.SetGlobal(meter)
		return meter
	}
	meter := metrics.New("mail-service", cfg.App.Env)
	metrics.SetGlobal(meter)
	return meter
}

// InvokeRunMetricsServer starts the operational metrics listener.
func InvokeRunMetricsServer(lc fx.Lifecycle, cfg *config.Config, meter metrics.Meter, lg logging.Logger) {
	var cancel context.CancelFunc
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			runCtx, runCancel := context.WithCancel(context.Background())
			cancel = runCancel
			go func() {
				if err := metrics.StartServer(runCtx, metrics.Config(cfg.Metrics), meter, lg); err != nil {
					lg.Error("metrics server stopped with error", logging.Err(err))
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

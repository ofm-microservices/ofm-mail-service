package appfx

import (
	"github.com/ofm-microseervices/ofm-common/pkg/logging"

	"go.uber.org/fx"
)

// AppModule wires application-start logging into the FX lifecycle.
var AppModule = fx.Options(
	fx.Invoke(InvokeStartLog),
)

// InvokeStartLog emits the process start log entry for mail-service.
func InvokeStartLog(lg logging.Logger) {
	lg.Info("starting mail-service")
}

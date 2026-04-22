package appfx

import "go.uber.org/fx"

// Module aggregates the full FX graph for mail-service.
var Module = fx.Options(
	ConfigModule,
	LoggerModule,
	AppModule,
	MessagingModule,
	ServiceModule,
	PresentationModule,
)

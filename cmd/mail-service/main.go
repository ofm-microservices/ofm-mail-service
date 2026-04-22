package main

import (
	appfx "mail-service/internal/fx"

	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		appfx.ConfigModule,
		appfx.LoggerModule,
		appfx.AppModule,
		appfx.MessagingModule,
		appfx.ServiceModule,
		appfx.PresentationModule,
	)
	app.Run()
}

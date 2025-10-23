package main

import (
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
	app := bootstrap.NewApp(
		bootstrap.HealthCheck(),
		// bootstrap.Tasks(),
		bootstrap.Sessions(time.Hour),
	)
	app.Use(bootstrap.NatsBus())
	app.Start()
}

package main

import (
	"time"

	"github.com/DotNetAge/sparrow/pkg/bootstrap"
)

func main() {
	app := bootstrap.NewApp(
		bootstrap.UseHealtCheck(),
		bootstrap.UseTasks(),
		bootstrap.UseSession(time.Hour),
	)
	app.Start()
}

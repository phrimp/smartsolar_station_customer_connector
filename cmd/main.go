package main

import (
	"fmt"
	"os"

	"station_customer_connector/internal/app"
	_ "station_customer_connector/internal/commands"
)

func main() {
	if err := app.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	app.Start()
}

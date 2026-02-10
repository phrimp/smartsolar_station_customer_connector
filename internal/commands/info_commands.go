package commands

import (
	"fmt"

	"station_customer_connector/internal/app"
)

func init() {
	app.RegisterCommandWithMetadata(
		versionCommand,
		"version",
		"Display application version information",
		"version",
	)

	app.RegisterCommandWithMetadata(
		statusCommand,
		"status",
		"Display database connection status",
		"status",
	)
}

func versionCommand(args string) error {
	fmt.Println("Station Customer Connector CLI")
	fmt.Println("Version: 1.5.0")
	fmt.Println("Go Version: 1.25.6")
	return nil
}

func statusCommand(args string) error {
	fmt.Println("Database Connection Status:")
	fmt.Println()

	if app.EnergyDB != nil {
		sqlDB, err := app.EnergyDB.DB()
		if err != nil {
			fmt.Printf("  EnergyDB: Error getting database instance: %v\n", err)
		} else if err := sqlDB.Ping(); err != nil {
			fmt.Printf("  EnergyDB: Connection failed: %v\n", err)
		} else {
			stats := sqlDB.Stats()
			fmt.Printf("  EnergyDB: Connected (Open: %d, Idle: %d, InUse: %d)\n",
				stats.OpenConnections, stats.Idle, stats.InUse)
		}
	} else {
		fmt.Println("  EnergyDB: Not initialized")
	}

	if app.CustomerDB != nil {
		sqlDB, err := app.CustomerDB.DB()
		if err != nil {
			fmt.Printf("  CustomerDB: Error getting database instance: %v\n", err)
		} else if err := sqlDB.Ping(); err != nil {
			fmt.Printf("  CustomerDB: Connection failed: %v\n", err)
		} else {
			stats := sqlDB.Stats()
			fmt.Printf("  CustomerDB: Connected (Open: %d, Idle: %d, InUse: %d)\n",
				stats.OpenConnections, stats.Idle, stats.InUse)
		}
	} else {
		fmt.Println("  CustomerDB: Not initialized")
	}

	return nil
}

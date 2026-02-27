package main

import (
	"fmt"
	"os"
	"time"

	"station_customer_connector/internal/database"
	"station_customer_connector/internal/models"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	EnergyDB   *gorm.DB
	CustomerDB *gorm.DB
)

func init() {
	godotenv.Load()

	var err error
	EnergyDB, err = initializeEnergyDB()
	if err != nil {
		fmt.Println("Error connecting to EnergyDB:", err)
	}
	CustomerDB, err = initializeCustomerDB()
	if err != nil {
		fmt.Println("Error connecting to CustomerDB:", err)
	}
}

func initializeEnergyDB() (*gorm.DB, error) {
	logLevel := gormlogger.Silent

	return database.NewPostgres(database.PostgresConfig{
		URL:             os.Getenv("ENERGY_DATABASE_URL"),
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: 10 * time.Second,
		LogLevel:        logLevel,
	})
}

func initializeCustomerDB() (*gorm.DB, error) {
	logLevel := gormlogger.Silent

	return database.NewPostgres(database.PostgresConfig{
		URL:             os.Getenv("CUSTOMER_DATABASE_URL"),
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: 10 * time.Second,
		LogLevel:        logLevel,
	})
}

func main() {
	var appendices []models.InstallationAppendix
	customerID := "13609ebd-9949-49e0-a322-4a69cd962bf3"

	err := CustomerDB.
		Joins("JOIN customers ON customers.id = installation_appendices.customer_id").
		Where("installation_appendices.customer_id = ?", customerID).
		Find(&appendices).Error

	fmt.Println(err, appendices)
}

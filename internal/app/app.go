package app

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"station_customer_connector/internal/database"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	EnergyDB   *gorm.DB
	CustomerDB *gorm.DB
	Commands   map[string]Command
)

type Command func(args string) error

type CommandMetadata struct {
	Handler     Command
	Description string
	Usage       string
}

var commandRegistry = make(map[string]*CommandMetadata)

func RegisterCommand(command Command, name string) {
	if Commands == nil {
		Commands = make(map[string]Command)
	}
	Commands[name] = command
	commandRegistry[name] = &CommandMetadata{
		Handler:     command,
		Description: "No description available",
		Usage:       name,
	}
}

func RegisterCommandWithMetadata(command Command, name, description, usage string) {
	if Commands == nil {
		Commands = make(map[string]Command)
	}
	Commands[name] = command
	commandRegistry[name] = &CommandMetadata{
		Handler:     command,
		Description: description,
		Usage:       usage,
	}
}

func Init() error {
	godotenv.Load()

	// Initialize command registry
	Commands = make(map[string]Command)

	// Register built-in commands
	registerBuiltInCommands()

	var err error
	EnergyDB, err = initializeEnergyDB()
	if err != nil {
		return err
	}
	CustomerDB, err = initializeCustomerDB()
	if err != nil {
		return err
	}
	return nil
}

func registerBuiltInCommands() {
	RegisterCommandWithMetadata(
		helpCommand,
		"help",
		"Display all available commands",
		"help [command]",
	)

	RegisterCommandWithMetadata(
		exitCommand,
		"exit",
		"Exit the CLI application",
		"exit",
	)

	RegisterCommandWithMetadata(
		exitCommand,
		"quit",
		"Exit the CLI application",
		"quit",
	)
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

func Start() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Welcome to Station Customer Connector CLI")
	fmt.Println("Type 'help' for available commands, 'exit' to quit")
	fmt.Println()

	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return
		}

		// Clean and parse input
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Parse command and arguments
		parts := strings.SplitN(input, " ", 2)
		commandName := parts[0]
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}

		// Execute command
		if err := executeCommand(commandName, args); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Println()
	}
}

func executeCommand(name string, args string) error {
	metadata, exists := commandRegistry[name]
	if !exists {
		return fmt.Errorf("unknown command: %s (type 'help' for available commands)", name)
	}

	return metadata.Handler(args)
}

func helpCommand(args string) error {
	if args != "" {
		// Show help for specific command
		metadata, exists := commandRegistry[args]
		if !exists {
			return fmt.Errorf("unknown command: %s", args)
		}
		fmt.Printf("Command: %s\n", args)
		fmt.Printf("Description: %s\n", metadata.Description)
		fmt.Printf("Usage: %s\n", metadata.Usage)
		return nil
	}

	// Show all commands
	fmt.Println("Available Commands:")
	fmt.Println()

	// Sort commands alphabetically
	commands := make([]string, 0, len(commandRegistry))
	for name := range commandRegistry {
		commands = append(commands, name)
	}
	sort.Strings(commands)

	// Display commands with descriptions
	for _, name := range commands {
		metadata := commandRegistry[name]
		fmt.Printf("  %-15s %s\n", name, metadata.Description)
	}

	fmt.Println()
	fmt.Println("Type 'help <command>' for detailed usage information")
	return nil
}

func exitCommand(args string) error {
	fmt.Println("Goodbye!")
	os.Exit(0)
	return nil
}

func GetCommandList() []string {
	commands := make([]string, 0, len(commandRegistry))
	for name := range commandRegistry {
		commands = append(commands, name)
	}
	sort.Strings(commands)
	return commands
}

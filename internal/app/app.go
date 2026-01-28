package app

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"station_customer_connector/internal/database"

	"github.com/joho/godotenv"
	"github.com/peterh/liner"
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
		fmt.Println("Error connecting to EnergyDB:", err)
	}
	CustomerDB, err = initializeCustomerDB()
	if err != nil {
		fmt.Println("Error connecting to CustomerDB:", err)
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

// InitializeEnergyDB initializes the EnergyDB connection (exported for retry functionality)
func InitializeEnergyDB() (*gorm.DB, error) {
	return initializeEnergyDB()
}

// InitializeCustomerDB initializes the CustomerDB connection (exported for retry functionality)
func InitializeCustomerDB() (*gorm.DB, error) {
	return initializeCustomerDB()
}

func Start() {
	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(false) // Don't abort on first Ctrl+C

	// Set up tab completion for commands
	line.SetCompleter(func(line string) (c []string) {
		for _, cmd := range GetCommandList() {
			if strings.HasPrefix(cmd, strings.ToLower(line)) {
				c = append(c, cmd)
			}
		}
		return
	})

	// Load command history from file if it exists
	historyFile := os.ExpandEnv("$HOME/.station_connector_history")
	if f, err := os.Open(historyFile); err == nil {
		line.ReadHistory(f)
		f.Close()
	}

	// Save history on exit
	defer func() {
		if f, err := os.Create(historyFile); err == nil {
			line.WriteHistory(f)
			f.Close()
		}
	}()

	// Set up signal handling for double Ctrl+C detection
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT)

	// Track last Ctrl+C time for double-tap detection
	var lastInterrupt time.Time
	var interruptMux sync.Mutex
	const doubleCtrlCWindow = 2 * time.Second

	// Goroutine to handle Ctrl+C signals
	go func() {
		for range sigChan {
			interruptMux.Lock()
			now := time.Now()
			timeSinceLastInterrupt := now.Sub(lastInterrupt)

			if timeSinceLastInterrupt < doubleCtrlCWindow {
				// Double Ctrl+C detected - exit
				interruptMux.Unlock()
				fmt.Println("\n\nReceived second Ctrl+C, exiting...")
				line.Close()
				os.Exit(0)
			} else {
				// First Ctrl+C - liner will clear the input automatically
				lastInterrupt = now
				interruptMux.Unlock()
				fmt.Println("\n^C (Input cleared. Press Ctrl+C again within 2s to exit)")
			}
		}
	}()

	fmt.Println("Welcome to Station Customer Connector CLI")
	fmt.Println("Type 'help' for available commands, 'exit' to quit")
	fmt.Println("Press Ctrl+C to clear input, press twice within 2s to exit")
	fmt.Println("Use arrow keys for history, Tab for command completion")
	fmt.Println()

	for {
		input, err := line.Prompt("> ")
		if err != nil {
			if err == liner.ErrPromptAborted {
				// Ctrl+C was pressed, continue to next prompt
				continue
			}
			// EOF or other error
			fmt.Println()
			return
		}

		// Add to history (only non-empty commands)
		trimmedInput := strings.TrimSpace(input)
		if trimmedInput != "" {
			line.AppendHistory(trimmedInput)
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

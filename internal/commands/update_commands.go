package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"station_customer_connector/internal/app"
)

func init() {
	app.RegisterCommandWithMetadata(
		updateCommand,
		"update",
		"Update the application from git repository",
		"update",
	)

	app.RegisterCommandWithMetadata(
		retryDBCommand,
		"retry-db",
		"Retry database connections for EnergyDB and CustomerDB",
		"retry-db",
	)
}

func updateCommand(args string) error {
	fmt.Println("Starting update process...")

	// Determine the operating system and script type
	isWindows := runtime.GOOS == "windows"

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}

	// Check if .git directory exists
	gitDir := filepath.Join(cwd, ".git")
	hasGitDir := false
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		hasGitDir = true
	}

	// Determine update strategy
	var updateStrategy string
	var scriptContent string
	var scriptPath string
	var scriptCmd *exec.Cmd

	defaultRepo := "https://github.com/phrimp/smartsolar_station_customer_connector"

	if hasGitDir {
		// Check if remote repository exists
		hasRemote, err := checkGitRemote()
		if err != nil {
			fmt.Printf("Warning: Failed to check git remote: %v\n", err)
			hasRemote = false
		}

		if hasRemote {
			updateStrategy = "git pull"
			fmt.Println("Found git repository with remote, will use 'git pull'")
		} else {
			updateStrategy = "git pull from default"
			fmt.Printf("Found git repository without remote, will pull from: %s\n", defaultRepo)
		}
	} else {
		updateStrategy = "clone"
		fmt.Printf("No git repository found, will clone from: %s\n", defaultRepo)
	}

	// Generate script based on OS
	if isWindows {
		scriptPath = filepath.Join(os.TempDir(), "update_script.ps1")
		scriptContent = generatePowerShellScript(updateStrategy, defaultRepo, cwd)

		// Create PowerShell script
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
			return fmt.Errorf("failed to create update script: %v", err)
		}

		fmt.Printf("Created update script: %s\n", scriptPath)
		fmt.Println("Executing update...")

		// Execute PowerShell script
		scriptCmd = exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	} else {
		scriptPath = filepath.Join(os.TempDir(), "update_script.sh")
		scriptContent = generateBashScript(updateStrategy, defaultRepo, cwd)

		// Create Bash script
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
			return fmt.Errorf("failed to create update script: %v", err)
		}

		fmt.Printf("Created update script: %s\n", scriptPath)
		fmt.Println("Executing update...")

		// Execute Bash script
		scriptCmd = exec.Command("bash", scriptPath)
	}

	// Set up command output
	scriptCmd.Stdout = os.Stdout
	scriptCmd.Stderr = os.Stderr
	scriptCmd.Dir = cwd

	// Execute the script
	if err := scriptCmd.Run(); err != nil {
		// Attempt to clean up script even if execution failed
		os.Remove(scriptPath)
		return fmt.Errorf("update script failed: %v", err)
	}

	// Clean up script
	fmt.Println("\nCleaning up...")
	if err := os.Remove(scriptPath); err != nil {
		fmt.Printf("Warning: Failed to delete script file %s: %v\n", scriptPath, err)
	} else {
		fmt.Println("Update script deleted successfully")
	}

	fmt.Println("\nUpdate completed! Please restart the application to use the new version.")
	fmt.Println("Type 'exit' to quit.")

	return nil
}

func checkGitRemote() (bool, error) {
	cmd := exec.Command("git", "remote", "-v")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	// If output is not empty, we have remotes
	return len(strings.TrimSpace(string(output))) > 0, nil
}

func generateBashScript(strategy, defaultRepo, workDir string) string {
	script := `#!/bin/bash
set -e

echo "Update Script Started"
echo "Working directory: ` + workDir + `"
echo ""

`

	switch strategy {
	case "git pull":
		script += `echo "Pulling latest changes from remote repository..."
git pull
if [ $? -eq 0 ]; then
    echo "Successfully pulled latest changes"
else
    echo "Failed to pull changes"
    exit 1
fi
`
	case "git pull from default":
		script += `echo "Adding remote repository: ` + defaultRepo + `"
git remote add origin "` + defaultRepo + `" 2>/dev/null || git remote set-url origin "` + defaultRepo + `"
echo "Pulling latest changes..."
git pull origin master || git pull origin main
if [ $? -eq 0 ]; then
    echo "Successfully pulled latest changes"
else
    echo "Failed to pull changes"
    exit 1
fi
`
	case "clone":
		script += `echo "Error: Cannot clone into existing directory"
echo "Please run this command from an empty directory or a git repository"
exit 1
`
	}

	script += `
echo ""
echo "Update completed successfully"
`

	return script
}

func generatePowerShellScript(strategy, defaultRepo, workDir string) string {
	script := `# PowerShell Update Script
$ErrorActionPreference = "Stop"

Write-Host "Update Script Started"
Write-Host "Working directory: ` + workDir + `"
Write-Host ""

`

	switch strategy {
	case "git pull":
		script += `try {
    Write-Host "Pulling latest changes from remote repository..."
    git pull
    if ($LASTEXITCODE -ne 0) {
        throw "Git pull failed"
    }
    Write-Host "Successfully pulled latest changes"
} catch {
    Write-Host "Error: $_"
    exit 1
}
`
	case "git pull from default":
		script += `try {
    Write-Host "Adding remote repository: ` + defaultRepo + `"
    git remote add origin "` + defaultRepo + `" 2>$null
    if ($LASTEXITCODE -ne 0) {
        git remote set-url origin "` + defaultRepo + `"
    }

    Write-Host "Pulling latest changes..."
    git pull origin master
    if ($LASTEXITCODE -ne 0) {
        git pull origin main
    }

    if ($LASTEXITCODE -ne 0) {
        throw "Git pull failed"
    }
    Write-Host "Successfully pulled latest changes"
} catch {
    Write-Host "Error: $_"
    exit 1
}
`
	case "clone":
		script += `Write-Host "Error: Cannot clone into existing directory"
Write-Host "Please run this command from an empty directory or a git repository"
exit 1
`
	}

	script += `
Write-Host ""
Write-Host "Update completed successfully"
`

	return script
}

func retryDBCommand(args string) error {
	fmt.Println("Retrying database connections...")
	fmt.Println()

	var energyErr, customerErr error
	successCount := 0

	// Retry EnergyDB connection
	if app.EnergyDB != nil {
		fmt.Println("Closing existing EnergyDB connection...")
		sqlDB, err := app.EnergyDB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}

	fmt.Println("Reconnecting to EnergyDB...")
	app.EnergyDB, energyErr = app.InitializeEnergyDB()
	if energyErr != nil {
		fmt.Printf("  EnergyDB: Failed to reconnect: %v\n", energyErr)
	} else {
		sqlDB, err := app.EnergyDB.DB()
		if err != nil {
			fmt.Printf("  EnergyDB: Error getting database instance: %v\n", err)
			energyErr = err
		} else if err := sqlDB.Ping(); err != nil {
			fmt.Printf("  EnergyDB: Connection test failed: %v\n", err)
			energyErr = err
		} else {
			stats := sqlDB.Stats()
			fmt.Printf("  EnergyDB: Successfully reconnected (Open: %d, Idle: %d, InUse: %d)\n",
				stats.OpenConnections, stats.Idle, stats.InUse)
			successCount++
		}
	}

	fmt.Println()

	// Retry CustomerDB connection
	if app.CustomerDB != nil {
		fmt.Println("Closing existing CustomerDB connection...")
		sqlDB, err := app.CustomerDB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}

	fmt.Println("Reconnecting to CustomerDB...")
	app.CustomerDB, customerErr = app.InitializeCustomerDB()
	if customerErr != nil {
		fmt.Printf("  CustomerDB: Failed to reconnect: %v\n", customerErr)
	} else {
		sqlDB, err := app.CustomerDB.DB()
		if err != nil {
			fmt.Printf("  CustomerDB: Error getting database instance: %v\n", err)
			customerErr = err
		} else if err := sqlDB.Ping(); err != nil {
			fmt.Printf("  CustomerDB: Connection test failed: %v\n", err)
			customerErr = err
		} else {
			stats := sqlDB.Stats()
			fmt.Printf("  CustomerDB: Successfully reconnected (Open: %d, Idle: %d, InUse: %d)\n",
				stats.OpenConnections, stats.Idle, stats.InUse)
			successCount++
		}
	}

	fmt.Println()
	fmt.Printf("Retry complete: %d of 2 database connections successful\n", successCount)

	if energyErr != nil && customerErr != nil {
		return fmt.Errorf("both database connections failed")
	}

	return nil
}

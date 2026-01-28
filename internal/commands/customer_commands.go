package functions

import (
	"fmt"
	"strings"
	"sync"

	"station_customer_connector/internal/app"
	"station_customer_connector/internal/models"
)

// Customer index cache for quick lookup by displayed index
var (
	customerCache   = make(map[int]models.Customer)
	customerCacheMu sync.RWMutex
)

func init() {
	app.RegisterCommandWithMetadata(
		ListCustomer,
		"ls-customer",
		"List customers with optional status filter",
		"ls-customer [--status=<status>]",
	)

	app.RegisterCommandWithMetadata(
		searchCustomersCommand,
		"search-customer",
		"Search customers by company name (case-insensitive)",
		"Usage: search-customer --query=<search_text>",
	)
}

func ListCustomer(args string) error {
	args_split := strings.Split(args, " ")
	var customers []models.Customer
	var total int64

	query := app.CustomerDB.Model(&models.Customer{})

	if len(args_split) > 1 {
		status := strings.Split(args_split[1], "=")[1]
		if status != "" {
			query = query.Where("status = ?", status)
		}

	}

	if err := query.Count(&total).Error; err != nil {
		return err
	}

	// Get paginated results
	err := query.
		Preload("Contracts").
		Preload("Contracts.InstallationAppendices").
		Preload("Contracts.InstallationAppendices.StationAssociations").
		Order("created_at DESC").
		Find(&customers).Error

	// Update customer cache
	customerCacheMu.Lock()
	customerCache = make(map[int]models.Customer)
	for idx, customer := range customers {
		customerCache[idx+1] = customer
	}
	customerCacheMu.Unlock()

	fmt.Println("Customer found:", total)
	for idx, customer := range customers {
		fmt.Printf("[%d] Name: %s --- Stations: \n", idx+1, customer.CompanyName)
		for _, contract := range customer.Contracts {
			for _, ia := range contract.InstallationAppendices {
				for _, station := range ia.StationAssociations {
					fmt.Printf("    Station ID: %s \n", station.ID)
				}
			}
		}
		fmt.Println("----------------------END of this Customer----------------------")
	}

	return err
}

// searchCustomersCommand searches customers by company name
func searchCustomersCommand(args string) error {
	// Parse --query argument
	query := ""
	for arg := range strings.FieldsSeq(args) {
		if after, ok := strings.CutPrefix(arg, "--query="); ok {
			query = after
			break
		}
	}

	if query == "" {
		return fmt.Errorf("missing --query argument. Usage: search-customer --query=<search_text>")
	}

	var customers []models.Customer

	// Search using LIKE with wildcards (case-insensitive)
	searchPattern := "%" + query + "%"
	if err := app.CustomerDB.
		Where("LOWER(customer_name) LIKE LOWER(?)", searchPattern).
		Order("customer_name ASC").
		Find(&customers).Error; err != nil {
		return fmt.Errorf("failed to search customers: %w", err)
	}

	if len(customers) == 0 {
		fmt.Printf("No customers found matching '%s'.\n", query)
		return nil
	}

	// Update customer cache
	customerCacheMu.Lock()
	customerCache = make(map[int]models.Customer)
	for idx, customer := range customers {
		customerCache[idx+1] = customer
	}
	customerCacheMu.Unlock()

	// Print header
	fmt.Println("\n========================================================================================")
	fmt.Printf("%-6s %-36s %-30s %-15s\n", "INDEX", "ID", "COMPANY NAME", "STATUS")
	fmt.Println("========================================================================================")

	// Print customers
	for idx, customer := range customers {
		companyName := customer.CompanyName
		if len(companyName) > 28 {
			companyName = companyName[:28] + ".."
		}

		fmt.Printf("%-6d %-36s %-30s %-15s\n", idx+1, customer.ID.String(), companyName, customer.Status)
	}

	fmt.Println("========================================================================================")
	fmt.Printf("Found: %d customers matching '%s' (cached for linking)\n\n", len(customers), query)

	return nil
}

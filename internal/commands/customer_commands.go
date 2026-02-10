package commands

import (
	"fmt"
	"strings"
	"sync"

	"station_customer_connector/internal/app"
	"station_customer_connector/internal/models"

	"github.com/google/uuid"
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

	app.RegisterCommandWithMetadata(
		deleteCustomerCommand,
		"delete-customer",
		"Delete a customer and all associated data (Contracts, Appendices, Links)",
		"Usage: delete-customer <customer_id>",
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
		billingName := ""
		if customer.BillingName != nil {
			billingName = *customer.BillingName
		}
		fmt.Printf("[%d] Name: %s --- Billing Name: %s \n", idx+1, customer.CompanyName, billingName)
		for _, contract := range customer.Contracts {
			for _, ia := range contract.InstallationAppendices {
				for _, assoc := range ia.StationAssociations {
					var st models.Station
					stationName := "Unknown"
					if err := app.EnergyDB.Model(&models.Station{}).Select("name").Where("id = ?", assoc.StationID).First(&st).Error; err == nil {
						stationName = st.Name
					}
					fmt.Printf("    Station ID: %s | Name: %s\n", assoc.StationID, stationName)
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

// deleteCustomerCommand deletes a customer and all associated data
func deleteCustomerCommand(args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		return fmt.Errorf("usage: delete-customer <customer_id_or_index>")
	}

	// Determine if input is an index or UUID
	var customerID uuid.UUID
	var err error

	// Try to parse as index first
	var index int
	if _, errScan := fmt.Sscanf(args, "%d", &index); errScan == nil {
		// Look up in cache
		customerCacheMu.RLock()
		customer, exists := customerCache[index]
		customerCacheMu.RUnlock()

		if !exists {
			return fmt.Errorf("customer index %d not found in cache. Run 'ls-customer' or 'search-customer' first", index)
		}
		customerID = customer.ID
		fmt.Printf("Targeting customer index %d: %s (%s)\n", index, customer.CompanyName, customer.ID)
	} else {
		// Try to parse as UUID
		customerID, err = uuid.Parse(args)
		// If both fail, return error
		if err != nil {
			// If it's not a UUID and not an index, maybe they passed "--id=" or something, but let's just stick to simple args first
			// check if it is "id=..."
			if strings.HasPrefix(args, "id=") {
				idStr := strings.TrimPrefix(args, "id=")
				customerID, err = uuid.Parse(idStr)
			}
		}

		if err != nil {
			return fmt.Errorf("invalid customer ID or index: %w", err)
		}
	}

	// Confirm deletion
	// Since CLI runs in non-interactive mode usually for automation, we might skip confirmation or just Log it.
	// But let's verify existence first.
	var customer models.Customer
	if err := app.CustomerDB.First(&customer, "id = ?", customerID).Error; err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	fmt.Printf("Deleting customer: %s (%s)\n", customer.CompanyName, customer.ID)

	// Begin Transaction
	tx := app.CustomerDB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// 1. Delete InstallationAppendixStation links
	// We need to find all appendices for this customer to delete their station links
	var appendixIDs []uuid.UUID
	if err := tx.Model(&models.InstallationAppendix{}).
		Select("id").
		Where("customer_id = ?", customerID).
		Find(&appendixIDs).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to fetch appendix IDs: %w", err)
	}

	if len(appendixIDs) > 0 {
		if err := tx.Where("installation_appendix_id IN ?", appendixIDs).
			Delete(&models.InstallationAppendixStation{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete station links: %w", err)
		}
		fmt.Printf("  - Deleted station links for %d appendices\n", len(appendixIDs))
	}

	// 2. Delete Installation Appendices
	if err := tx.Where("customer_id = ?", customerID).Delete(&models.InstallationAppendix{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete installation appendices: %w", err)
	}
	fmt.Println("  - Deleted installation appendices")

	// 3. Delete Contracts
	if err := tx.Where("customer_id = ?", customerID).Delete(&models.Contract{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete contracts: %w", err)
	}
	fmt.Println("  - Deleted contracts")

	// 4. Delete Customer Contacts
	if err := tx.Where("customer_id = ?", customerID).Delete(&models.CustomerContact{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete customer contacts: %w", err)
	}
	fmt.Println("  - Deleted customer contacts")

	// 5. Delete Customer Documents
	if err := tx.Where("customer_id = ?", customerID).Delete(&models.CustomerDocument{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete customer documents: %w", err)
	}
	fmt.Println("  - Deleted customer documents")

	// 6. Delete Monitoring App Login Information
	if err := tx.Where("customer_id = ?", customerID).Delete(&models.MonitoringAppLoginInformation{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete monitoring app login info: %w", err)
	}
	fmt.Println("  - Deleted monitoring app login info")

	// 7. Delete Customer
	if err := tx.Delete(&customer).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete customer: %w", err)
	}

	// Commit Transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("✓ Customer deleted successfully")
	return nil
}

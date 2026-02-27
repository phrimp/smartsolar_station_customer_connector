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
		"List customers with optional status filter and monitoring login info",
		"ls-customer [--status=<status>] [--with-login-info]",
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

	app.RegisterCommandWithMetadata(
		addMonitoringLoginCommand,
		"add-monitoring-login",
		"Add monitoring app login information to a customer",
		"Usage: add-monitoring-login <customer_index> --name=<name> --url=<url> --username=<username> --password=<password>",
	)

	app.RegisterCommandWithMetadata(
		removeMonitoringLoginCommand,
		"remove-monitoring-login",
		"Remove monitoring app login information by ID",
		"Usage: remove-monitoring-login <login_info_id>",
	)

	app.RegisterCommandWithMetadata(
		listMonitoringLoginCommand,
		"ls-monitoring-login",
		"List all monitoring login information for a customer",
		"Usage: ls-monitoring-login <customer_index_or_id>",
	)
}

func ListCustomer(args string) error {
	args_split := strings.Split(args, " ")
	var customers []models.Customer
	var total int64
	withLoginInfo := false

	query := app.CustomerDB.Model(&models.Customer{})

	// Parse arguments
	for _, arg := range args_split {
		if strings.HasPrefix(arg, "--status=") {
			status := strings.TrimPrefix(arg, "--status=")
			if status != "" {
				query = query.Where("status = ?", status)
			}
		}
		if arg == "--with-login-info" {
			withLoginInfo = true
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return err
	}

	// Get paginated results
	query = query.
		Preload("Contracts").
		Preload("Contracts.InstallationAppendices").
		Preload("Contracts.InstallationAppendices.StationAssociations")

	// Conditionally preload MonitoringAppLoginInformationList
	if withLoginInfo {
		query = query.Preload("MonitoringAppLoginInformationList")
	}

	err := query.
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

		// Display monitoring login information if requested
		if withLoginInfo && len(customer.MonitoringAppLoginInformationList) > 0 {
			fmt.Println("    Monitoring App Login Information:")
			for _, login := range customer.MonitoringAppLoginInformationList {
				name := "N/A"
				if login.Name != nil {
					name = *login.Name
				}
				url := "N/A"
				if login.AppURL != nil {
					url = *login.AppURL
				}
				username := "N/A"
				if login.Username != nil {
					username = *login.Username
				}
				password := "****" // Masked for security
				if login.Password != nil && *login.Password != "" {
					password = "****"
				}
				fmt.Printf("      - Name: %s | URL: %s | Username: %s | Password: %s\n", name, url, username, password)
			}
		}

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

// addMonitoringLoginCommand adds new monitoring app login information to a customer
func addMonitoringLoginCommand(args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		return fmt.Errorf("usage: add-monitoring-login <customer_id_or_index> --name=<name> --url=<url> --username=<username> --password=<password>")
	}

	// Parse arguments
	argsSplit := strings.Fields(args)
	if len(argsSplit) < 1 {
		return fmt.Errorf("missing customer ID or index")
	}

	customerIDOrIndex := argsSplit[0]
	var customerID uuid.UUID
	var err error

	// Try to parse as index first
	var index int
	if _, errScan := fmt.Sscanf(customerIDOrIndex, "%d", &index); errScan == nil {
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
		customerID, err = uuid.Parse(customerIDOrIndex)
		if err != nil {
			return fmt.Errorf("invalid customer ID or index: %w", err)
		}
	}

	// Verify customer exists
	var customer models.Customer
	if err := app.CustomerDB.First(&customer, "id = ?", customerID).Error; err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	// Parse login information fields
	var name, appURL, username, password string
	for _, arg := range argsSplit[1:] {
		if strings.HasPrefix(arg, "--name=") {
			name = strings.TrimPrefix(arg, "--name=")
		} else if strings.HasPrefix(arg, "--url=") {
			appURL = strings.TrimPrefix(arg, "--url=")
		} else if strings.HasPrefix(arg, "--username=") {
			username = strings.TrimPrefix(arg, "--username=")
		} else if strings.HasPrefix(arg, "--password=") {
			password = strings.TrimPrefix(arg, "--password=")
		}
	}

	// Validate required fields (all fields are optional in the schema, but we should ask for at least some info)
	if name == "" && appURL == "" && username == "" && password == "" {
		return fmt.Errorf("at least one field must be provided: --name, --url, --username, or --password")
	}

	// Create new MonitoringAppLoginInformation
	loginInfo := models.MonitoringAppLoginInformation{
		CustomerID: &customerID,
	}

	if name != "" {
		loginInfo.Name = &name
	}
	if appURL != "" {
		loginInfo.AppURL = &appURL
	}
	if username != "" {
		loginInfo.Username = &username
	}
	if password != "" {
		loginInfo.Password = &password
	}

	// Save to database
	if err := app.CustomerDB.Create(&loginInfo).Error; err != nil {
		return fmt.Errorf("failed to create monitoring login info: %w", err)
	}

	fmt.Printf("✓ Monitoring login information added successfully\n")
	fmt.Printf("  ID: %s\n", loginInfo.ID)
	fmt.Printf("  Customer: %s\n", customer.CompanyName)
	if name != "" {
		fmt.Printf("  Name: %s\n", name)
	}
	if appURL != "" {
		fmt.Printf("  URL: %s\n", appURL)
	}
	if username != "" {
		fmt.Printf("  Username: %s\n", username)
	}
	if password != "" {
		fmt.Printf("  Password: ****\n")
	}

	return nil
}

// listMonitoringLoginCommand lists all monitoring login information for a customer
func listMonitoringLoginCommand(args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		return fmt.Errorf("usage: ls-monitoring-login <customer_id_or_index>")
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
		fmt.Printf("Listing monitoring login info for customer index %d: %s (%s)\n\n", index, customer.CompanyName, customer.ID)
	} else {
		// Try to parse as UUID
		customerID, err = uuid.Parse(args)
		if err != nil {
			return fmt.Errorf("invalid customer ID or index: %w", err)
		}
	}

	// Verify customer exists and fetch login information
	var customer models.Customer
	if err := app.CustomerDB.Preload("MonitoringAppLoginInformationList").First(&customer, "id = ?", customerID).Error; err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	if len(customer.MonitoringAppLoginInformationList) == 0 {
		fmt.Println("No monitoring login information found for this customer.")
		return nil
	}

	// Print header
	fmt.Println("========================================================================================")
	fmt.Printf("%-38s %-20s %-30s %-20s\n", "ID", "NAME", "URL", "USERNAME")
	fmt.Println("========================================================================================")

	// Print monitoring login information
	for _, login := range customer.MonitoringAppLoginInformationList {
		name := "N/A"
		if login.Name != nil {
			name = *login.Name
			if len(name) > 18 {
				name = name[:18] + ".."
			}
		}

		url := "N/A"
		if login.AppURL != nil {
			url = *login.AppURL
			if len(url) > 28 {
				url = url[:28] + ".."
			}
		}

		username := "N/A"
		if login.Username != nil {
			username = *login.Username
			if len(username) > 18 {
				username = username[:18] + ".."
			}
		}

		fmt.Printf("%-38s %-20s %-30s %-20s\n", login.ID.String(), name, url, username)
	}

	fmt.Println("========================================================================================")
	fmt.Printf("Total: %d monitoring login records\n\n", len(customer.MonitoringAppLoginInformationList))

	return nil
}

// removeMonitoringLoginCommand removes a monitoring app login information record
func removeMonitoringLoginCommand(args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		return fmt.Errorf("usage: remove-monitoring-login <login_info_id>")
	}

	// Parse login info ID
	loginInfoID, err := uuid.Parse(args)
	if err != nil {
		return fmt.Errorf("invalid monitoring login info ID: %w", err)
	}

	// Verify the record exists and fetch it for display
	var loginInfo models.MonitoringAppLoginInformation
	if err := app.CustomerDB.Preload("Customer").First(&loginInfo, "id = ?", loginInfoID).Error; err != nil {
		return fmt.Errorf("monitoring login information not found: %w", err)
	}

	// Display what will be deleted
	customerName := "Unknown"
	if loginInfo.Customer != nil {
		customerName = loginInfo.Customer.CompanyName
	}

	name := "N/A"
	if loginInfo.Name != nil {
		name = *loginInfo.Name
	}

	url := "N/A"
	if loginInfo.AppURL != nil {
		url = *loginInfo.AppURL
	}

	username := "N/A"
	if loginInfo.Username != nil {
		username = *loginInfo.Username
	}

	fmt.Printf("Deleting monitoring login information:\n")
	fmt.Printf("  ID: %s\n", loginInfo.ID)
	fmt.Printf("  Customer: %s\n", customerName)
	fmt.Printf("  Name: %s\n", name)
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  Username: %s\n", username)
	fmt.Println()

	// Delete the record
	if err := app.CustomerDB.Delete(&loginInfo).Error; err != nil {
		return fmt.Errorf("failed to delete monitoring login info: %w", err)
	}

	fmt.Println("✓ Monitoring login information deleted successfully")
	return nil
}

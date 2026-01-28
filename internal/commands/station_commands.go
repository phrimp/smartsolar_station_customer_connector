package functions

import (
	"fmt"
	"maps"
	"strings"
	"sync"

	"station_customer_connector/internal/app"
	"station_customer_connector/internal/models"

	"github.com/google/uuid"
)

// LinkCache stores station_id -> customer_id mappings in memory
type LinkCache struct {
	mu    sync.RWMutex
	links map[uuid.UUID]uuid.UUID // map[station_id]customer_id
}

var linkCache = &LinkCache{
	links: make(map[uuid.UUID]uuid.UUID),
}

// Index caches for quick lookup by displayed index
var (
	stationCache   = make(map[int]models.Station)
	stationCacheMu sync.RWMutex
)

// Add adds or updates a station-customer link
func (lc *LinkCache) Add(stationID, customerID uuid.UUID) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.links[stationID] = customerID
}

// Remove removes a station-customer link
func (lc *LinkCache) Remove(stationID uuid.UUID) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	delete(lc.links, stationID)
}

// Get retrieves a customer ID for a station
func (lc *LinkCache) Get(stationID uuid.UUID) (uuid.UUID, bool) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	customerID, exists := lc.links[stationID]
	return customerID, exists
}

// GetAll returns a copy of all links
func (lc *LinkCache) GetAll() map[uuid.UUID]uuid.UUID {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	result := make(map[uuid.UUID]uuid.UUID, len(lc.links))
	maps.Copy(result, lc.links)
	return result
}

// Clear removes all links
func (lc *LinkCache) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.links = make(map[uuid.UUID]uuid.UUID)
}

// Count returns the number of links
func (lc *LinkCache) Count() int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return len(lc.links)
}

func init() {
	app.RegisterCommandWithMetadata(
		listStationsCommand,
		"ls-station",
		"List all stations with index, ID, name, and billing name",
		"Usage: ls-station",
	)

	app.RegisterCommandWithMetadata(
		searchStationsCommand,
		"search-station",
		"Search stations by name or billing name (case-insensitive)",
		"Usage: search-station --query=<search_text>",
	)

	app.RegisterCommandWithMetadata(
		linkStationCustomerCommand,
		"link",
		"Link a station to a customer (adds to cache)",
		"Usage: link <station_index> <customer_index> OR link --station_id=<uuid> --customer_id=<uuid>",
	)

	app.RegisterCommandWithMetadata(
		showLinksCommand,
		"show-links",
		"Display current station-customer links in cache",
		"Usage: show-links",
	)

	app.RegisterCommandWithMetadata(
		clearLinksCommand,
		"clear-links",
		"Clear all links from cache",
		"Usage: clear-links",
	)

	app.RegisterCommandWithMetadata(
		commitLinksCommand,
		"commit-links",
		"Validate and commit all links to database",
		"Usage: commit-links",
	)
}

// listStationsCommand lists all stations with index, ID, name, and billing name
func listStationsCommand(args string) error {
	var stations []models.Station

	// Fetch all stations ordered by name
	if err := app.EnergyDB.Order("name ASC").Find(&stations).Error; err != nil {
		return fmt.Errorf("failed to fetch stations: %w", err)
	}

	if len(stations) == 0 {
		fmt.Println("No stations found.")
		return nil
	}

	// Update station cache
	stationCacheMu.Lock()
	stationCache = make(map[int]models.Station)
	for idx, station := range stations {
		stationCache[idx+1] = station
	}
	stationCacheMu.Unlock()

	// Print header
	fmt.Println("\n========================================================================================================")
	fmt.Printf("%-6s %-36s %-30s %-30s\n", "INDEX", "ID", "NAME", "BILLING NAME")
	fmt.Println("========================================================================================================")

	// Print stations
	for idx, station := range stations {
		billingName := "N/A"
		if station.BillingName != nil && *station.BillingName != "" {
			billingName = *station.BillingName
		}

		name := station.Name
		if len(name) > 28 {
			name = name[:28] + ".."
		}
		if len(billingName) > 28 {
			billingName = billingName[:28] + ".."
		}

		fmt.Printf("%-6d %-36s %-30s %-30s\n", idx+1, station.ID.String(), name, billingName)
	}

	fmt.Println("========================================================================================================")
	fmt.Printf("Total: %d stations (cached for linking)\n\n", len(stations))

	return nil
}

// searchStationsCommand searches stations by name or billing name
func searchStationsCommand(args string) error {
	// Parse --query argument
	query := ""
	for arg := range strings.FieldsSeq(args) {
		if after, ok := strings.CutPrefix(arg, "--query="); ok {
			query = after
			break
		}
	}

	if query == "" {
		return fmt.Errorf("missing --query argument. Usage: search-station --query=<search_text>")
	}

	var stations []models.Station

	// Search using LIKE with wildcards (case-insensitive)
	searchPattern := "%" + query + "%"
	if err := app.EnergyDB.
		Where("LOWER(name) LIKE LOWER(?) OR LOWER(billing_name) LIKE LOWER(?)", searchPattern, searchPattern).
		Order("name ASC").
		Find(&stations).Error; err != nil {
		return fmt.Errorf("failed to search stations: %w", err)
	}

	if len(stations) == 0 {
		fmt.Printf("No stations found matching '%s'.\n", query)
		return nil
	}

	// Update station cache
	stationCacheMu.Lock()
	stationCache = make(map[int]models.Station)
	for idx, station := range stations {
		stationCache[idx+1] = station
	}
	stationCacheMu.Unlock()

	// Print header
	fmt.Println("\n========================================================================================================")
	fmt.Printf("%-6s %-36s %-30s %-30s\n", "INDEX", "ID", "NAME", "BILLING NAME")
	fmt.Println("========================================================================================================")

	// Print stations
	for idx, station := range stations {
		billingName := "N/A"
		if station.BillingName != nil && *station.BillingName != "" {
			billingName = *station.BillingName
		}

		name := station.Name
		if len(name) > 28 {
			name = name[:28] + ".."
		}
		if len(billingName) > 28 {
			billingName = billingName[:28] + ".."
		}

		fmt.Printf("%-6d %-36s %-30s %-30s\n", idx+1, station.ID.String(), name, billingName)
	}

	fmt.Println("========================================================================================================")
	fmt.Printf("Found: %d stations matching '%s' (cached for linking)\n\n", len(stations), query)

	return nil
}

// linkStationCustomerCommand adds a station-customer link to cache
func linkStationCustomerCommand(args string) error {
	args = strings.TrimSpace(args)

	if args == "" {
		return fmt.Errorf("usage: link <station_index> <customer_index> OR link --station_id=<uuid> --customer_id=<uuid>")
	}

	// Check if using index-based syntax (numeric arguments)
	fields := strings.Fields(args)

	// Index-based syntax: link 1 2
	if len(fields) == 2 && !strings.HasPrefix(fields[0], "--") {
		return linkByIndex(fields[0], fields[1])
	}

	// UUID-based syntax: link --station_id=xxx --customer_id=yyy
	return linkByUUID(args)
}

// linkByIndex links station and customer using their display indices
func linkByIndex(stationIndexStr, customerIndexStr string) error {
	// Parse indices
	var stationIndex, customerIndex int
	if _, err := fmt.Sscanf(stationIndexStr, "%d", &stationIndex); err != nil {
		return fmt.Errorf("invalid station index '%s': must be a number", stationIndexStr)
	}
	if _, err := fmt.Sscanf(customerIndexStr, "%d", &customerIndex); err != nil {
		return fmt.Errorf("invalid customer index '%s': must be a number", customerIndexStr)
	}

	// Lookup station from cache
	stationCacheMu.RLock()
	station, stationExists := stationCache[stationIndex]
	stationCacheMu.RUnlock()

	if !stationExists {
		return fmt.Errorf("station index %d not found in cache. Run 'ls-station' or 'search-station' first", stationIndex)
	}

	// Lookup customer from cache
	customerCacheMu.RLock()
	customer, customerExists := customerCache[customerIndex]
	customerCacheMu.RUnlock()

	if !customerExists {
		return fmt.Errorf("customer index %d not found in cache. Run 'ls-customer' or 'search-customer' first", customerIndex)
	}

	// Add to link cache
	linkCache.Add(station.ID, customer.ID)

	fmt.Printf("✓ Linked station #%d '%s' to customer #%d '%s'\n",
		stationIndex, station.Name, customerIndex, customer.CompanyName)
	fmt.Printf("Total links in cache: %d\n", linkCache.Count())

	return nil
}

// linkByUUID links station and customer using their UUIDs
func linkByUUID(args string) error {
	// Parse arguments
	var stationIDStr, customerIDStr string
	for arg := range strings.FieldsSeq(args) {
		if after, ok := strings.CutPrefix(arg, "--station_id="); ok {
			stationIDStr = after
		} else if after0, ok0 := strings.CutPrefix(arg, "--customer_id="); ok0 {
			customerIDStr = after0
		}
	}

	if stationIDStr == "" {
		return fmt.Errorf("missing --station_id argument. Usage: link --station_id=<uuid> --customer_id=<uuid>")
	}

	if customerIDStr == "" {
		return fmt.Errorf("missing --customer_id argument. Usage: link --station_id=<uuid> --customer_id=<uuid>")
	}

	// Parse station ID
	stationID, err := uuid.Parse(stationIDStr)
	if err != nil {
		return fmt.Errorf("invalid station_id format: %w", err)
	}

	// Validate station exists
	var station models.Station
	if err := app.EnergyDB.Where("id = ?", stationID).First(&station).Error; err != nil {
		return fmt.Errorf("station not found: %w", err)
	}

	// Parse customer ID (can be empty string)
	var customerID uuid.UUID
	if customerIDStr != "" && customerIDStr != "\"\"" && customerIDStr != "''" {
		customerID, err = uuid.Parse(customerIDStr)
		if err != nil {
			return fmt.Errorf("invalid customer_id format: %w", err)
		}

		// Validate customer exists
		var customer models.Customer
		if err := app.CustomerDB.Where("id = ?", customerID).First(&customer).Error; err != nil {
			return fmt.Errorf("customer not found: %w", err)
		}
	}

	// Add to cache
	linkCache.Add(stationID, customerID)

	if customerID == uuid.Nil {
		fmt.Printf("✓ Linked station '%s' (ID: %s) to <empty customer>\n", station.Name, stationID)
	} else {
		var customer models.Customer
		app.CustomerDB.Where("id = ?", customerID).First(&customer)
		fmt.Printf("✓ Linked station '%s' (ID: %s) to customer '%s' (ID: %s)\n",
			station.Name, stationID, customer.CompanyName, customerID)
	}

	fmt.Printf("Total links in cache: %d\n", linkCache.Count())

	return nil
}

// showLinksCommand displays all current links in cache
func showLinksCommand(args string) error {
	links := linkCache.GetAll()

	if len(links) == 0 {
		fmt.Println("No links in cache.")
		return nil
	}

	fmt.Println("\n========================================================================================================")
	fmt.Printf("%-6s %-36s %-30s %-30s\n", "INDEX", "STATION ID", "STATION NAME", "CUSTOMER NAME")
	fmt.Println("========================================================================================================")

	idx := 1
	for stationID, customerID := range links {
		var station models.Station
		stationName := "Unknown"
		if err := app.EnergyDB.Where("id = ?", stationID).First(&station).Error; err == nil {
			stationName = station.Name
		}

		customerName := "<empty>"
		if customerID != uuid.Nil {
			var customer models.Customer
			if err := app.CustomerDB.Where("id = ?", customerID).First(&customer).Error; err == nil {
				customerName = customer.CompanyName
			}
		}

		if len(stationName) > 28 {
			stationName = stationName[:28] + ".."
		}
		if len(customerName) > 28 {
			customerName = customerName[:28] + ".."
		}

		fmt.Printf("%-6d %-36s %-30s %-30s\n", idx, stationID.String(), stationName, customerName)
		idx++
	}

	fmt.Println("========================================================================================================")
	fmt.Printf("Total: %d links in cache\n\n", len(links))

	return nil
}

// clearLinksCommand removes all links from cache
func clearLinksCommand(args string) error {
	count := linkCache.Count()
	linkCache.Clear()
	fmt.Printf("✓ Cleared %d links from cache\n", count)
	return nil
}

// commitLinksCommand validates and commits all links to database
func commitLinksCommand(args string) error {
	links := linkCache.GetAll()

	if len(links) == 0 {
		return fmt.Errorf("no links in cache to commit")
	}

	fmt.Printf("Validating %d links...\n\n", len(links))

	// Step 1: Validate all stations have customer_id
	var invalidLinks []uuid.UUID
	for stationID, customerID := range links {
		if customerID == uuid.Nil {
			invalidLinks = append(invalidLinks, stationID)
		}
	}

	if len(invalidLinks) > 0 {
		fmt.Println("❌ Validation failed. The following stations have no customer_id:")
		for _, stationID := range invalidLinks {
			var station models.Station
			stationName := "Unknown"
			if err := app.EnergyDB.Where("id = ?", stationID).First(&station).Error; err == nil {
				stationName = station.Name
			}
			fmt.Printf("  - %s (ID: %s)\n", stationName, stationID)
		}
		return fmt.Errorf("all stations must have a customer_id before committing")
	}

	fmt.Println("✓ All stations have valid customer_id")

	// Step 2: Find InstallationAppendixID for each customer
	type LinkCommit struct {
		StationID              uuid.UUID
		CustomerID             uuid.UUID
		InstallationAppendixID uuid.UUID
		StationName            string
		CustomerName           string
	}

	var commits []LinkCommit
	var missingAppendix []uuid.UUID

	for stationID, customerID := range links {
		var station models.Station
		if err := app.EnergyDB.Where("id = ?", stationID).First(&station).Error; err != nil {
			return fmt.Errorf("failed to fetch station %s: %w", stationID, err)
		}

		var customer models.Customer
		if err := app.CustomerDB.Where("id = ?", customerID).First(&customer).Error; err != nil {
			return fmt.Errorf("failed to fetch customer %s: %w", customerID, err)
		}

		// Find installation appendix for this customer
		// Path: Customer -> Contracts -> InstallationAppendices
		var appendices []models.InstallationAppendix
		if err := app.CustomerDB.
			Joins("JOIN contracts ON contracts.id = installation_appendices.contract_id").
			Where("contracts.customer_id = ?", customerID).
			Where("installation_appendices.status = ?", "active").
			Find(&appendices).Error; err != nil {
			return fmt.Errorf("failed to query installation appendices for customer %s: %w", customerID, err)
		}

		if len(appendices) == 0 {
			missingAppendix = append(missingAppendix, customerID)
			fmt.Printf("  ⚠ Customer '%s' has no active installation appendix\n", customer.CompanyName)
			continue
		}

		// Use the first active appendix
		commits = append(commits, LinkCommit{
			StationID:              stationID,
			CustomerID:             customerID,
			InstallationAppendixID: appendices[0].ID,
			StationName:            station.Name,
			CustomerName:           customer.CompanyName,
		})
	}

	if len(missingAppendix) > 0 {
		return fmt.Errorf("some customers have no active installation appendix. Please create appendices first")
	}

	fmt.Printf("✓ Found installation appendices for all %d customers\n\n", len(commits))

	// Step 3: Insert/update records in InstallationAppendixStation
	fmt.Println("Committing to database...")

	successCount := 0
	for _, commit := range commits {
		// Check if link already exists
		var existing models.InstallationAppendixStation
		err := app.CustomerDB.
			Where("installation_appendix_id = ? AND station_id = ?", commit.InstallationAppendixID, commit.StationID).
			First(&existing).Error

		if err == nil {
			// Link already exists, skip
			fmt.Printf("  ⊙ Link already exists: %s -> %s\n", commit.StationName, commit.CustomerName)
			successCount++
			continue
		}

		// Create new link
		newLink := models.InstallationAppendixStation{
			ID:                     uuid.New(),
			InstallationAppendixID: commit.InstallationAppendixID,
			StationID:              commit.StationID,
		}

		if err := app.CustomerDB.Create(&newLink).Error; err != nil {
			fmt.Printf("  ❌ Failed to create link for station '%s': %v\n", commit.StationName, err)
			continue
		}

		fmt.Printf("  ✓ Created link: %s -> %s\n", commit.StationName, commit.CustomerName)
		successCount++
	}

	fmt.Printf("\n✓ Successfully committed %d/%d links\n", successCount, len(commits))

	// Step 4: Clear cache
	linkCache.Clear()
	fmt.Println("✓ Cache cleared")

	return nil
}

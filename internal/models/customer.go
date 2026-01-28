package models

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

type CustomerStatus string

const (
	CustomerStatusDraft       CustomerStatus = "draft"
	CustomerStatusUnderReview CustomerStatus = "under_review"
	CustomerStatusApproved    CustomerStatus = "approved"
	CustomerStatusRejected    CustomerStatus = "rejected"
	CustomerStatusInactive    CustomerStatus = "inactive"
)

// Customer represents a business customer (SME) using SmartSolar services
// RELAXED SCHEMA: Most fields optional to support iterative development
type Customer struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`

	// Basic Information (Relaxed - only customer_name required)
	CompanyName       string   `gorm:"column:customer_name;size:255;not null" json:"customer_name"` // Changed to customer_name per relaxed schema
	TaxCode           *string  `gorm:"column:tax_code;size:20" json:"tax_code,omitempty"`           // RELAXED: Now nullable
	BusinessLicense   *string  `gorm:"column:business_license;size:50" json:"business_license,omitempty"`
	BusinessAddress   *string  `gorm:"column:business_address;type:text" json:"business_address,omitempty"`
	RegisteredCapital *float64 `gorm:"column:registered_capital;type:decimal(15,2)" json:"registered_capital,omitempty"`
	IncorporationDate *string  `gorm:"column:incorporation_date;type:date" json:"incorporation_date,omitempty"`
	IndustryType      *string  `gorm:"column:industry_type;size:100" json:"industry_type,omitempty"`
	BillingName       *string  `gorm:"column:billing_name;size:255" json:"billing_name"`

	// Contact Info (from relaxed schema - inline contact fields)
	ContactName  *string `gorm:"column:contact_name;size:255" json:"contact_name,omitempty"`
	ContactTitle *string `gorm:"column:contact_title;size:100" json:"contact_title,omitempty"`
	ContactEmail *string `gorm:"column:contact_email;size:255" json:"contact_email,omitempty"`
	ContactPhone *string `gorm:"column:contact_phone;size:20" json:"contact_phone,omitempty"`

	// EVN Pricing
	EVNPricePlanID *uuid.UUID `gorm:"column:evn_price_plan_id;type:uuid" json:"evn_price_plan_id,omitempty"`

	// Workflow & Management
	Status           CustomerStatus `gorm:"column:status;size:20;default:'active'" json:"status"`                    // CHANGED: Default is 'active' not 'draft'
	Source           *string        `gorm:"column:source;size:100" json:"source,omitempty"`                          // NEW field
	AccountManagerID *uuid.UUID     `gorm:"column:account_manager_id;type:uuid" json:"account_manager_id,omitempty"` // Changed from sales_person_id

	// Extended Info (from relaxed schema)
	OwnershipStructure JSONB   `gorm:"column:ownership_structure;type:jsonb" json:"ownership_structure,omitempty"`
	DirectorName       *string `gorm:"column:director_name;size:255" json:"director_name,omitempty"`

	// Credit (Relaxed)
	CreditStatus string `gorm:"column:credit_status;size:20;default:'pending'" json:"credit_status"`

	// Metadata
	Notes     *string    `gorm:"column:notes;type:text" json:"notes,omitempty"`
	CreatedAt time.Time  `gorm:"column:created_at;default:now()" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at;default:now()" json:"updated_at"`
	CreatedBy *uuid.UUID `gorm:"column:created_by_user_id;type:uuid" json:"created_by_user_id,omitempty"` // Changed column name

	// Relationships (Separate tables still exist)
	Contacts                          []CustomerContact               `gorm:"foreignKey:CustomerID" json:"contacts,omitempty"`
	Documents                         []CustomerDocument              `gorm:"foreignKey:CustomerID" json:"documents,omitempty"`
	Contracts                         []Contract                      `gorm:"foreignKey:CustomerID" json:"contracts,omitempty"`
	MonitoringAppLoginInformationList []MonitoringAppLoginInformation `gorm:"foreignKey:CustomerID" json:"monitoring_app_login_information_list,omitempty"`
	EVNPricePlan                      *EVNPricePlan                   `gorm:"foreignKey:EVNPricePlanID" json:"evn_price_plan,omitempty"`
}

// TableName specifies the table name for Customer
func (Customer) TableName() string {
	return "customers"
}

// IsApproved checks if customer is approved
func (c *Customer) IsApproved() bool {
	return c.Status == CustomerStatusApproved
}

// CanTransition checks if status transition is valid
func (c *Customer) CanTransition(newStatus CustomerStatus) bool {
	validTransitions := map[CustomerStatus][]CustomerStatus{
		CustomerStatusDraft:       {CustomerStatusUnderReview},
		CustomerStatusUnderReview: {CustomerStatusApproved, CustomerStatusRejected, CustomerStatusDraft},
		CustomerStatusApproved:    {CustomerStatusInactive},
		CustomerStatusRejected:    {CustomerStatusDraft},
		CustomerStatusInactive:    {CustomerStatusApproved},
	}

	allowedTransitions, ok := validTransitions[c.Status]
	if !ok {
		return false
	}

	return slices.Contains(allowedTransitions, newStatus)
}

// CustomerContact represents a contact person for a customer
// RELAXED SCHEMA: Simplified structure
type CustomerContact struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`
	CustomerID *uuid.UUID `gorm:"column:customer_id;type:uuid" json:"customer_id,omitempty"` // RELAXED: Now nullable

	Name      *string `gorm:"column:name;size:255" json:"name,omitempty"` // RELAXED: Now nullable
	Email     *string `gorm:"column:email;size:255" json:"email,omitempty"`
	WorkPhone *string `gorm:"column:work_phone;size:20" json:"work_phone,omitempty"`
	Mobile    *string `gorm:"column:mobile;size:20" json:"mobile,omitempty"`
	IsPrimary bool    `gorm:"column:is_primary;default:false" json:"is_primary"`
}

// TableName specifies the table name for CustomerContact
func (CustomerContact) TableName() string {
	return "customer_contacts"
}

// CustomerDocument represents a document uploaded for a customer
// RELAXED SCHEMA: Minimal required fields
type CustomerDocument struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`
	CustomerID *uuid.UUID `gorm:"column:customer_id;type:uuid" json:"customer_id,omitempty"` // RELAXED: Now nullable

	DocumentType *string    `gorm:"column:document_type;size:100" json:"document_type,omitempty"` // RELAXED
	FilePath     *string    `gorm:"column:file_path;size:500" json:"file_path,omitempty"`         // RELAXED
	UploadedAt   *time.Time `gorm:"column:uploaded_at;default:now()" json:"uploaded_at,omitempty"`
}

// TableName specifies the table name for CustomerDocument
func (CustomerDocument) TableName() string {
	return "customer_documents"
}

// ContractType represents the type of solar contract
type ContractType string

const (
	ContractTypeEquipmentLeasing ContractType = "equipment_leasing"
	ContractTypeInstallment      ContractType = "installment"
	ContractTypePPA              ContractType = "ppa"
)

// ContractStatus represents the status of a contract
type ContractStatus string

const (
	ContractStatusDraft      ContractStatus = "draft"
	ContractStatusActive     ContractStatus = "active"
	ContractStatusSuspended  ContractStatus = "suspended"
	ContractStatusTerminated ContractStatus = "terminated"
)

// Contract represents a solar lease agreement
// RELAXED SCHEMA: Most fields optional except contract_number and customer_id
type Contract struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`
	CustomerID uuid.UUID  `gorm:"column:customer_id;type:uuid;not null" json:"customer_id"`
	SalesRepID *uuid.UUID `gorm:"column:sales_rep_id;type:uuid" json:"sales_rep_id,omitempty"`

	// Contract Identification (contract_number required for reference)
	ContractNumber string  `gorm:"column:contract_number;size:50;unique;not null" json:"contract_number"`
	ContractType   *string `gorm:"column:contract_type;size:50;default:'equipment_leasing'" json:"contract_type,omitempty"` // RELAXED

	// Contract Dates (RELAXED - can be NULL in draft)
	SigningDate   *string `gorm:"column:signing_date;type:date" json:"signing_date,omitempty"`
	EffectiveDate *string `gorm:"column:effective_date;type:date" json:"effective_date,omitempty"`

	// Status
	Status          ContractStatus `gorm:"column:status;size:20;default:'draft'" json:"status"`
	TerminationDate *string        `gorm:"column:termination_date;type:date" json:"termination_date,omitempty"`

	// Payment Terms (with defaults)
	PaymentDueDays     *int     `gorm:"column:payment_due_days;default:5" json:"payment_due_days,omitempty"`
	LatePaymentFeeRate *float64 `gorm:"column:late_payment_fee_rate;type:decimal(5,4);default:0.0010" json:"late_payment_fee_rate,omitempty"`

	// Metadata
	CreatedAt time.Time `gorm:"column:created_at;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;default:now()" json:"updated_at"`

	// Relationships
	Customer               *Customer              `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	InstallationAppendices []InstallationAppendix `gorm:"foreignKey:ContractID" json:"installation_appendices,omitempty"`
}

// TableName specifies the table name for Contract
func (Contract) TableName() string {
	return "contracts"
}

// IsActive checks if contract is active
func (c *Contract) IsActive() bool {
	return c.Status == ContractStatusActive
}

// InstallationAppendix represents a solar installation site (billing unit)
// RELAXED SCHEMA: Tech specs and dates optional to allow iterative entry
type InstallationAppendix struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`
	ContractID       uuid.UUID  `gorm:"column:contract_id;type:uuid;not null" json:"contract_id"`
	CustomerID       uuid.UUID  `gorm:"column:customer_id;type:uuid;not null" json:"customer_id"`
	ProjectManagerID *uuid.UUID `gorm:"column:project_manager_id;type:uuid" json:"project_manager_id,omitempty"`

	// Identification (appendix_number required for reference)
	AppendixNumber string     `gorm:"column:appendix_number;size:50;unique;not null" json:"appendix_number"`
	StationID      *uuid.UUID `gorm:"column:station_id;type:uuid" json:"station_id,omitempty"` // Link to external monitoring DEPRECATED

	// Site Info (RELAXED)
	InstallationAddress *string  `gorm:"column:installation_address;type:text" json:"installation_address,omitempty"`
	InstallationArea    *float64 `gorm:"column:installation_area;type:decimal(10,2)" json:"installation_area,omitempty"`

	// Tech Specs (RELAXED - needed for billing, but optional for entry)
	SystemCapacityKwp *float64 `gorm:"column:system_capacity_kwp;type:decimal(10,2)" json:"system_capacity_kwp,omitempty"`
	ServiceFeeRate    *float64 `gorm:"column:service_fee_rate;type:decimal(5,2)" json:"service_fee_rate,omitempty"`
	MinimumMonthlyFee *float64 `gorm:"column:minimum_monthly_fee;type:decimal(12,2)" json:"minimum_monthly_fee,omitempty"` // Auto-calculated

	// Tech Details (RELAXED)
	InverterBrand   *string `gorm:"column:inverter_brand;size:100" json:"inverter_brand,omitempty"`
	PanelBrand      *string `gorm:"column:panel_brand;size:100" json:"panel_brand,omitempty"`
	BatteryIncluded bool    `gorm:"column:battery_included;default:false" json:"battery_included"`

	// Dates (RELAXED)
	ConstructionStartDate *string `gorm:"column:construction_start_date;type:date" json:"construction_start_date,omitempty"`
	HandoverDate          *string `gorm:"column:handover_date;type:date" json:"handover_date,omitempty"`                 // Triggers billing
	MinimumTermEndDate    *string `gorm:"column:minimum_term_end_date;type:date" json:"minimum_term_end_date,omitempty"` // Auto-calculated

	// Status
	Status string `gorm:"column:status;size:20;default:'planned'" json:"status"`

	// Monitoring (RELAXED)
	MonitoringPlatform *string `gorm:"column:monitoring_platform;size:50" json:"monitoring_platform,omitempty"`
	MonitoringUsername *string `gorm:"column:monitoring_username;size:100" json:"monitoring_username,omitempty"`

	// Metadata
	CreatedAt time.Time `gorm:"column:created_at;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;default:now()" json:"updated_at"`

	// Relationships
	Contract            *Contract                     `gorm:"foreignKey:ContractID" json:"contract,omitempty"`
	Customer            *Customer                     `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	StationAssociations []InstallationAppendixStation `gorm:"foreignKey:InstallationAppendixID" json:"station_associations,omitempty"`

	// DEPRECATED: Cross-database relationship - kept for backward compatibility but should not be used
	// Use StationAssociations instead and fetch Station data separately via energy module API
}

// TableName specifies the table name for InstallationAppendix
func (InstallationAppendix) TableName() string {
	return "installation_appendices"
}

// InstallationAppendixStation represents the junction table between installation appendices and stations
// NOTE: This is a reference-only table. The actual Station records exist in a different database (energy module)
// We store only the station_id UUID without foreign key constraints
type InstallationAppendixStation struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`
	InstallationAppendixID uuid.UUID `gorm:"column:installation_appendix_id;type:uuid;not null" json:"installation_appendix_id"`
	StationID              uuid.UUID `gorm:"column:station_id;type:uuid;not null;unique" json:"station_id"` // Reference to energy.stations (different DB)
	CreatedAt              time.Time `gorm:"column:created_at;default:now()" json:"created_at"`

	// Relationship to InstallationAppendix only (within same database)
	InstallationAppendix *InstallationAppendix `gorm:"foreignKey:InstallationAppendixID" json:"installation_appendix,omitempty"`
}

// TableName specifies the table name for InstallationAppendixStation
func (InstallationAppendixStation) TableName() string {
	return "installation_appendix_stations"
}

// JSONB is a type alias for map used in JSONB columns
type JSONB map[string]interface{}

// EVNPricePlan represents EVN (Vietnam Electricity) price plans
type EVNPricePlan struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`

	Name          *string  `gorm:"column:name;type:text" json:"name,omitempty"`
	PeakPrice     *float64 `gorm:"column:peak_price;type:decimal(7,4)" json:"peak_price,omitempty"`     // VND per kWh during peak hours
	NormalPrice   *float64 `gorm:"column:normal_price;type:decimal(7,4)" json:"normal_price,omitempty"` // VND per kWh during normal hours
	EffectiveDate *string  `gorm:"column:effective_date;type:date;default:current_date" json:"effective_date,omitempty"`
	EndDate       *string  `gorm:"column:end_date;type:date" json:"end_date,omitempty"`
}

// TableName specifies the table name for EVNPricePlan
func (EVNPricePlan) TableName() string {
	return "evn_price_plan"
}

// IsActive checks if the price plan is currently active
func (e *EVNPricePlan) IsActive() bool {
	// TODO: Implement date-based active check when dates are stored as time.Time
	// For now, if EndDate is nil, it's considered active
	return e.EndDate == nil
}

// MonitoringAppLoginInformation represents login credentials for monitoring applications
// RELAXED SCHEMA: All fields are nullable to support flexible entry
type MonitoringAppLoginInformation struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`
	CustomerID *uuid.UUID `gorm:"column:customer_id;type:uuid" json:"customer_id,omitempty"` // Foreign key to customers
	Name       *string    `gorm:"column:name;size:255" json:"name,omitempty"`                // Name/label for this login credential
	AppURL     *string    `gorm:"column:app_url;size:500" json:"app_url,omitempty"`
	Username   *string    `gorm:"column:username;size:500" json:"username,omitempty"`
	Password   *string    `gorm:"column:password;size:500" json:"password,omitempty"`

	// Relationship
	Customer *Customer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
}

// TableName specifies the table name for MonitoringAppLoginInformation
func (MonitoringAppLoginInformation) TableName() string {
	return "monitoring_app_login_information"
}

package models

import (
	"time"

	"github.com/google/uuid"
)

// Station represents a physical site/station
type Station struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id" json:"id"`
	ProviderID   string     `gorm:"column:provider_id;not null" json:"provider_id"`
	ExternalID   string     `gorm:"column:external_id;not null" json:"external_id"`
	Name         string     `gorm:"column:name;not null" json:"name"`
	BillingName  *string    `gorm:"column:billing_name" json:"billing_name,omitempty"`
	CustomerCode *string    `gorm:"column:customer_code" json:"customer_code,omitempty"`
	CustomerName *string    `gorm:"column:customer_name" json:"customer_name,omitempty"`
	Phone        *string    `gorm:"column:phone" json:"phone,omitempty"`
	Email        *string    `gorm:"column:email" json:"email,omitempty"`
	Address      *string    `gorm:"column:address" json:"address,omitempty"`
	Model        *string    `gorm:"column:model" json:"model,omitempty"`
	CapacityKw   *float64   `gorm:"type:numeric(10,2);column:capacity_kw" json:"capacity_kw,omitempty"`
	IsActive     bool       `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt    time.Time  `gorm:"column:created_at;default:now()" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;default:now()" json:"updated_at"`
	APICreatedAt *time.Time `gorm:"column:api_created_at" json:"api_created_at,omitempty"`
	DeviceCount  int        `gorm:"column:device_count;default:0" json:"device_count"`

	// Relationships
	Provider *Provider `gorm:"foreignKey:ProviderID;references:ID" json:"provider,omitempty"`
}

// TableName specifies the table name for Station
func (Station) TableName() string {
	return "stations"
}

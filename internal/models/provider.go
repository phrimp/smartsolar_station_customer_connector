package models

import "time"

type ProviderType string

const (
	ProviderTypeMeter    ProviderType = "meter"
	ProviderTypeInverter ProviderType = "inverter"
)

// Provider represents a data source provider (e.g., NewGelex, Sungrow, Deye, Solis)
type Provider struct {
	ID           string       `gorm:"primaryKey;column:id" json:"id"`
	DisplayName  string       `gorm:"column:display_name;not null" json:"display_name"`
	Type         ProviderType `gorm:"column:type;not null" json:"type"`
	BaseURL      string       `gorm:"column:base_url;not null" json:"base_url"`
	SecretName   string       `gorm:"column:secret_name;not null" json:"secret_name"`
	IsActive     bool         `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt    time.Time    `gorm:"column:created_at;default:now()" json:"created_at"`
	UpdatedAt    time.Time    `gorm:"column:updated_at;default:now()" json:"updated_at"`
	StationCount int          `gorm:"column:station_count;default:0" json:"station_count"`
}

// TableName specifies the table name for Provider
func (Provider) TableName() string {
	return "providers"
}

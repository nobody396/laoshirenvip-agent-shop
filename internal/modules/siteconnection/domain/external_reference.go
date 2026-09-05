package domain

import "time"

const (
	ExternalReferenceKindProduct = "product"
	ExternalReferenceKindSKU     = "sku"
)

// ExternalReference gives protocols with string identifiers a stable numeric
// identity without hashing. Dujiao's catalog mapping contracts currently use
// uint identifiers, so every external key is allocated once and persisted.
type ExternalReference struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	ConnectionID uint      `gorm:"not null;uniqueIndex:idx_external_reference_key" json:"connection_id"`
	Kind         string    `gorm:"type:varchar(16);not null;uniqueIndex:idx_external_reference_key" json:"kind"`
	ExternalKey  string    `gorm:"type:varchar(1024);not null;uniqueIndex:idx_external_reference_key" json:"external_key"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time `gorm:"index" json:"updated_at"`
}

func (ExternalReference) TableName() string { return "site_connection_external_references" }

// Package model provides shared GORM model fields. All primary keys use UUID v4 (char(36) / PostgreSQL uuid).
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// Base is the default embed for domain tables: UUID PK, timestamps, soft delete.
// Embed anonymously, e.g. `type Product struct { Base; Title string }`.
type Base struct {
	ID        uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate assigns a UUID when id is zero.
func (b *Base) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&b.ID)
	return nil
}

// HardDeleteBase is for tables without soft delete (logs, immutable facts).
type HardDeleteBase struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BeforeCreate assigns a UUID when id is zero.
func (b *HardDeleteBase) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&b.ID)
	return nil
}

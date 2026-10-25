package files

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// FileRecord is metadata for an uploaded object.
type FileRecord struct {
	ID             uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID       int64     `gorm:"not null;default:0;index" json:"tenantId"`
	OriginalName   string    `gorm:"size:512;not null" json:"filename"`
	ObjectKey      string    `gorm:"size:512;uniqueIndex;not null" json:"objectKey"`
	PublicURL      string    `gorm:"size:1024;not null" json:"url"`
	ContentType    string    `gorm:"size:128" json:"contentType"`
	Size           int64     `json:"size"`
	StorageKind    string    `gorm:"size:32;not null" json:"storageKind"`
	SecurityStatus string    `gorm:"size:32;not null;default:'pending_scan';index" json:"securityStatus"`
	ScanStatus     string    `gorm:"size:32;not null;default:'pending_scan';index" json:"scanStatus"`
	// ScanClaimID and ScanClaimedAt fence duplicate or recovered queue
	// reservations. They are internal coordination fields and are never part
	// of the public file metadata response.
	ScanClaimID   string     `gorm:"size:36;index" json:"-"`
	ScanClaimedAt *time.Time `gorm:"index" json:"-"`
	CreatedBy     *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// TableName keeps a stable table name for migrations.
func (FileRecord) TableName() string {
	return "files"
}

// BeforeCreate assigns a UUID when id is zero.
func (f *FileRecord) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&f.ID)
	return nil
}

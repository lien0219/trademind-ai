package admin

import (
 "time"

 "github.com/google/uuid"
)

// RootAccount is a singleton, durable binding to the original bootstrap
// identity. Granting role=admin or PlatformOperator never creates a Root.
// No HTTP endpoint may modify this record.
type RootAccount struct {
 ID int64 `gorm:"primaryKey;autoIncrement:false"`
 UserID uuid.UUID `gorm:"type:char(36);not null;uniqueIndex"`
 CreatedAt time.Time
}
func (RootAccount) TableName() string { return "root_accounts" }

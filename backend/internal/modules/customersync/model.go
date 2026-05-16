package customersync

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// CustomerMessageSyncTask records one shop customer message pull job.
type CustomerMessageSyncTask struct {
	model.HardDeleteBase
	ShopID       uuid.UUID      `gorm:"type:char(36);index;not null" json:"shopId"`
	Platform     string         `gorm:"size:64;index;not null" json:"platform"`
	TaskType     string         `gorm:"size:64;index;not null" json:"taskType"`
	Status       string         `gorm:"size:32;index;not null" json:"status"`
	Mode         string         `gorm:"size:32;index;not null" json:"mode"`
	Cursor       string         `gorm:"type:text" json:"cursor,omitempty"`
	StartedAt    *time.Time     `json:"startedAt,omitempty"`
	FinishedAt   *time.Time     `json:"finishedAt,omitempty"`
	TotalCount   int            `gorm:"default:0;not null" json:"totalCount"`
	SuccessCount int            `gorm:"default:0;not null" json:"successCount"`
	FailedCount  int            `gorm:"default:0;not null" json:"failedCount"`
	ErrorMessage string         `gorm:"type:text" json:"errorMessage,omitempty"`
	Input        datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output       datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	CreatedBy    *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	LockedBy     *string        `gorm:"size:220;index" json:"lockedBy,omitempty"`
	LockedUntil  *time.Time     `gorm:"index" json:"lockedUntil,omitempty"`
	LockVersion  int            `gorm:"default:0;not null" json:"lockVersion"`
}

func (CustomerMessageSyncTask) TableName() string { return "customer_message_sync_tasks" }

package collect

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Task status values (aligned with rules).
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
	StatusRetrying  = "retrying"
)

// CollectTask persists orchestration state; collector never writes this table.
type CollectTask struct {
	model.HardDeleteBase
	Source          string         `gorm:"size:64;index;not null" json:"source"`
	SourceURL       string         `gorm:"size:2048;not null" json:"sourceUrl"`
	Status          string         `gorm:"size:32;index;not null" json:"status"`
	ResultProductID *uuid.UUID     `gorm:"type:char(36);index" json:"resultProductId,omitempty"`
	RawResult       datatypes.JSON `gorm:"type:jsonb" json:"rawResult,omitempty"`
	ErrorMessage    string         `gorm:"type:text" json:"errorMessage,omitempty"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedAt       *time.Time     `json:"startedAt,omitempty"`
	FinishedAt      *time.Time     `json:"finishedAt,omitempty"`
}

func (CollectTask) TableName() string { return "collect_tasks" }

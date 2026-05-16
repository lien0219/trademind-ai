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

// Batch aggregate status (derived from child tasks via reconciliation).
const (
	BatchStatusRunning        = "running"
	BatchStatusPartialSuccess = "partial_success"
	BatchStatusSuccess        = "success"
	BatchStatusFailed         = "failed"
	BatchStatusCancelled      = "cancelled"
)

// CollectBatch groups many collect_tasks (e.g. bulk 1688 links).
type CollectBatch struct {
	model.HardDeleteBase
	Source         string     `gorm:"size:64;index;not null" json:"source"`
	TotalCount     int        `gorm:"not null" json:"totalCount"`
	PendingCount   int        `gorm:"not null" json:"pendingCount"`
	RunningCount   int        `gorm:"not null" json:"runningCount"`
	SuccessCount   int        `gorm:"not null" json:"successCount"`
	FailedCount    int        `gorm:"not null" json:"failedCount"`
	CancelledCount int        `gorm:"not null" json:"cancelledCount"`
	Status         string     `gorm:"size:32;index;not null" json:"status"`
	CreatedBy      *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
}

func (CollectBatch) TableName() string { return "collect_batches" }

// CollectTask persists orchestration state; collector never writes this table.
type CollectTask struct {
	model.HardDeleteBase
	BatchID         *uuid.UUID     `gorm:"type:char(36);index" json:"batchId,omitempty"`
	Source          string         `gorm:"size:64;index;not null" json:"source"`
	SourceURL       string         `gorm:"size:2048;not null" json:"sourceUrl"`
	Status          string         `gorm:"size:32;index;not null" json:"status"`
	ResultProductID *uuid.UUID     `gorm:"type:char(36);index" json:"resultProductId,omitempty"`
	RawResult       datatypes.JSON `gorm:"type:jsonb" json:"rawResult,omitempty"`
	ErrorMessage    string         `gorm:"type:text" json:"errorMessage,omitempty"`
	RetryCount      int            `gorm:"not null;default:0" json:"retryCount"`
	MaxRetries      int            `gorm:"not null;default:3" json:"maxRetries"`
	NextRetryAt     *time.Time     `json:"nextRetryAt,omitempty"`
	RetryEnqueuedAt *time.Time     `json:"retryEnqueuedAt,omitempty"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedAt       *time.Time     `json:"startedAt,omitempty"`
	FinishedAt      *time.Time     `json:"finishedAt,omitempty"`
}

func (CollectTask) TableName() string { return "collect_tasks" }

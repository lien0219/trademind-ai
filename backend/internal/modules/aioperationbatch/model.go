package aioperationbatch

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Batch operation kinds (persisted operation_type).
const (
	OperationTitleOptimize          = "title_optimize"
	OperationDescriptionGenerate    = "description_generate"
	OperationImageRemoveBackground  = "image_remove_background"
	OperationImageGenerateScene     = "image_generate_scene"
	OperationImageReplaceBackground = "image_replace_background"
)

// Batch aggregate status values.
const (
	StatusPending        = "pending"
	StatusRunning        = "running"
	StatusSuccess        = "success"
	StatusPartialSuccess = "partial_success"
	StatusFailed         = "failed"
	StatusCancelled      = "cancelled"
)

// AIOperationBatch groups bulk AI tasks (text or image orchestration metadata).
type AIOperationBatch struct {
	model.Base
	BatchNo       string         `gorm:"size:48;uniqueIndex;not null" json:"batchNo"`
	OperationType string         `gorm:"size:64;index;not null" json:"operationType"`
	Status        string         `gorm:"size:32;index;not null" json:"status"`
	ProductCount  int            `gorm:"not null;default:0" json:"productCount"`
	TaskCount     int            `gorm:"not null;default:0" json:"taskCount"`
	SuccessCount  int            `gorm:"not null;default:0" json:"successCount"`
	FailedCount   int            `gorm:"not null;default:0" json:"failedCount"`
	SkippedCount  int            `gorm:"not null;default:0" json:"skippedCount"`
	Input         datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output        datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	CreatedBy     *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedAt     *time.Time     `json:"startedAt,omitempty"`
	FinishedAt    *time.Time     `json:"finishedAt,omitempty"`
}

func (AIOperationBatch) TableName() string { return "ai_operation_batches" }

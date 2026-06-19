package productpublish

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

const (
	BatchPending        = "pending"
	BatchRunning        = "running"
	BatchSuccess        = "success"
	BatchPartialSuccess = "partial_success"
	BatchFailed         = "failed"

	CapRealDraftCreate = "real_draft_create"
	CapLocalDraftOnly  = "local_draft_only"
	CapNotConfigured   = "not_configured"
	CapNotAuthorized   = "not_authorized"
	CapDisabled        = "disabled"

	TaskTypeLocalDraftCreate = "local_draft_create"
)

// ProductPublishBatch groups multi-target draft creation for one product.
type ProductPublishBatch struct {
	model.HardDeleteBase
	ProductID    uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	Status       string         `gorm:"size:32;index;not null" json:"status"`
	TargetCount  int            `json:"targetCount"`
	ReadyCount   int            `json:"readyCount"`
	SuccessCount int            `json:"successCount"`
	FailedCount  int            `json:"failedCount"`
	WarningCount int            `json:"warningCount"`
	BlockedCount int            `json:"blockedCount"`
	Summary      datatypes.JSON `gorm:"type:jsonb" json:"summary,omitempty"`
	Input        datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	CreatedBy    *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	FinishedAt   *time.Time     `json:"finishedAt,omitempty"`
}

func (ProductPublishBatch) TableName() string { return "product_publish_batches" }

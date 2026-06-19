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
	BatchCancelled      = "cancelled"

	BatchTypeSingleProduct = "single_product"
	BatchTypeMultiProduct  = "multi_product"

	CapRealDraftCreate = "real_draft_create"
	CapLocalDraftOnly  = "local_draft_only"
	CapNotConfigured   = "not_configured"
	CapNotAuthorized   = "not_authorized"
	CapDisabled        = "disabled"

	TaskTypeLocalDraftCreate = "local_draft_create"
)

// ProductPublishBatch groups multi-target draft creation for one or many products.
type ProductPublishBatch struct {
	model.HardDeleteBase
	BatchType      string         `gorm:"size:32;index;default:single_product;not null" json:"batchType"`
	Name           string         `gorm:"size:256" json:"name,omitempty"`
	ProductID      *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	ProductCount   int            `json:"productCount"`
	TargetCount    int            `json:"targetCount"`
	TaskCount      int            `json:"taskCount"`
	ReadyCount     int            `json:"readyCount"`
	SuccessCount   int            `json:"successCount"`
	FailedCount    int            `json:"failedCount"`
	SkippedCount   int            `json:"skippedCount"`
	WarningCount   int            `json:"warningCount"`
	BlockedCount   int            `json:"blockedCount"`
	IdempotencyKey string         `gorm:"size:128;index" json:"-"`
	Summary        datatypes.JSON `gorm:"type:jsonb" json:"summary,omitempty"`
	Input          datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	CreatedBy      *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	FinishedAt     *time.Time     `json:"finishedAt,omitempty"`
}

func (ProductPublishBatch) TableName() string { return "product_publish_batches" }

package aiproductimage

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

const (
	BatchTypeAIImage = "ai_image"

	OpQualityCheck       = "quality_check"
	OpRemoveWatermark    = "remove_watermark"
	OpRemoveLogo         = "remove_logo"
	OpWhiteBackground    = "white_background"
	OpOptimizeBackground = "optimize_background"
	OpTranslateText      = "translate_text"
	OpSelectBestMain     = "select_best_main"

	ApplySetMain       = "set_main"
	ApplyAddDetail     = "add_detail"
	ApplyReplaceImage  = "replace_image"
	ApplySaveToGallery = "save_to_gallery"

	ItemPending       = "pending"
	ItemRunning       = "running"
	ItemSuccess       = "success"
	ItemFailed        = "failed"
	ItemPendingReview = "pending_review"
	ItemApplied       = "applied"
	ItemRejected      = "rejected"
	ItemConflict      = "conflict"
	ItemCancelled     = "cancelled"

	ConflictUserMessage = "商品图片在 AI 处理结果生成后已经被修改。为避免覆盖人工修改，请重新对比后再应用。"

	BatchPending        = "pending"
	BatchRunning        = "running"
	BatchSuccess        = "success"
	BatchPartialSuccess = "partial_success"
	BatchFailed         = "failed"
	BatchCancelled      = "cancelled"

	defaultMaxProducts = 50
	defaultMaxImages   = 300
)

// AIProductImageBatch groups bulk AI image processing with human review.
type AIProductImageBatch struct {
	model.Base
	BatchNo        string         `gorm:"size:48;uniqueIndex;not null" json:"batchNo"`
	BatchType      string         `gorm:"size:32;index;not null;default:ai_image" json:"batchType"`
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	ProductCount   int            `gorm:"not null;default:0" json:"productCount"`
	ImageCount     int            `gorm:"not null;default:0" json:"imageCount"`
	ItemCount      int            `gorm:"not null;default:0" json:"itemCount"`
	SuccessCount   int            `gorm:"not null;default:0" json:"successCount"`
	FailedCount    int            `gorm:"not null;default:0" json:"failedCount"`
	AppliedCount   int            `gorm:"not null;default:0" json:"appliedCount"`
	IdempotencyKey string         `gorm:"size:64;uniqueIndex" json:"idempotencyKey,omitempty"`
	Input          datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output         datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	CreatedBy      *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedAt      *time.Time     `json:"startedAt,omitempty"`
	FinishedAt     *time.Time     `json:"finishedAt,omitempty"`
}

func (AIProductImageBatch) TableName() string { return "ai_product_image_batches" }

// AIProductImageItem is one product image × operation sub-task with review state.
type AIProductImageItem struct {
	model.Base
	BatchID            uuid.UUID      `gorm:"type:char(36);index;not null" json:"batchId"`
	ProductID          uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	ImageID            *uuid.UUID     `gorm:"type:char(36);index" json:"imageId,omitempty"`
	ImageType          string         `gorm:"size:32;index" json:"imageType"`
	OperationType      string         `gorm:"size:32;index;not null" json:"operationType"`
	Status             string         `gorm:"size:32;index;not null" json:"status"`
	ImageTaskID        *uuid.UUID     `gorm:"type:char(36);index" json:"imageTaskId,omitempty"`
	SourceImageURL     string         `gorm:"type:text" json:"sourceImageUrl,omitempty"`
	SourceSnapshotHash string         `gorm:"size:128" json:"sourceSnapshotHash,omitempty"`
	ResultImageURL     string         `gorm:"type:text" json:"resultImageUrl,omitempty"`
	ResultStorageKey   string         `gorm:"size:512" json:"resultStorageKey,omitempty"`
	QualityWarnings    datatypes.JSON `gorm:"type:jsonb" json:"qualityWarnings,omitempty"`
	ApplyMode          string         `gorm:"size:32" json:"applyMode,omitempty"`
	ApplicationID      *uuid.UUID     `gorm:"type:char(36);index" json:"applicationId,omitempty"`
	ErrorCode          string         `gorm:"size:64" json:"errorCode,omitempty"`
	ErrorMessage       string         `gorm:"type:text" json:"errorMessage,omitempty"`
	AppliedAt          *time.Time     `json:"appliedAt,omitempty"`
	AppliedBy          *uuid.UUID     `gorm:"type:char(36)" json:"appliedBy,omitempty"`
	ImageUpdatedAt     *time.Time     `json:"imageUpdatedAt,omitempty"`
}

func (AIProductImageItem) TableName() string { return "ai_product_image_items" }

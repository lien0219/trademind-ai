package aiproducttext

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

const (
	BatchTypeAIText = "ai_text"

	OpTitle       = "title"
	OpDescription = "description"

	ItemPending       = "pending"
	ItemRunning       = "running"
	ItemSuccess       = "success"
	ItemFailed        = "failed"
	ItemPendingReview = "pending_review"
	ItemApplied       = "applied"
	ItemRejected      = "rejected"
	ItemConflict      = "conflict"
	ItemCancelled     = "cancelled"

	// ConflictUserMessage is shown when apply detects product content changed after AI generation.
	ConflictUserMessage = "商品内容在 AI 建议生成后已经被修改。为避免覆盖人工修改，请重新对比后再应用。"

	BatchPending        = "pending"
	BatchRunning        = "running"
	BatchSuccess        = "success"
	BatchPartialSuccess = "partial_success"
	BatchFailed         = "failed"
	BatchCancelled      = "cancelled"
)

// AIProductTextBatch groups bulk AI title/description generation with human review.
type AIProductTextBatch struct {
	model.Base
	BatchNo        string         `gorm:"size:48;uniqueIndex;not null" json:"batchNo"`
	BatchType      string         `gorm:"size:32;index;not null;default:ai_text" json:"batchType"`
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	ProductCount   int            `gorm:"not null;default:0" json:"productCount"`
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

func (AIProductTextBatch) TableName() string { return "ai_product_text_batches" }

// AIProductTextItem is one product × content-type sub-task with review state.
type AIProductTextItem struct {
	model.Base
	BatchID            uuid.UUID      `gorm:"type:char(36);index;not null" json:"batchId"`
	ProductID          uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	OperationType      string         `gorm:"size:32;index;not null" json:"operationType"`
	Status             string         `gorm:"size:32;index;not null" json:"status"`
	AITaskID           *uuid.UUID     `gorm:"type:char(36);index" json:"aiTaskId,omitempty"`
	SourceSnapshot     datatypes.JSON `gorm:"type:jsonb" json:"sourceSnapshot,omitempty"`
	SourceSnapshotHash string         `gorm:"size:128" json:"sourceSnapshotHash,omitempty"`
	GeneratedText      string         `gorm:"type:text" json:"generatedText,omitempty"`
	EditedText         string         `gorm:"type:text" json:"editedText,omitempty"`
	QualityWarnings    datatypes.JSON `gorm:"type:jsonb" json:"qualityWarnings,omitempty"`
	ErrorCode          string         `gorm:"size:64" json:"errorCode,omitempty"`
	ErrorMessage       string         `gorm:"type:text" json:"errorMessage,omitempty"`
	ApplicationID      *uuid.UUID     `gorm:"type:char(36);index" json:"applicationId,omitempty"`
	AppliedAt          *time.Time     `json:"appliedAt,omitempty"`
	AppliedBy          *uuid.UUID     `gorm:"type:char(36)" json:"appliedBy,omitempty"`
	ProductUpdatedAt   *time.Time     `json:"productUpdatedAt,omitempty"`
}

func (AIProductTextItem) TableName() string { return "ai_product_text_items" }

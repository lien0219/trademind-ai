package imagetask

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Task type constants.
const (
	TaskTypeRemoveBackground  = "remove_background"
	TaskTypeReplaceBackground = "replace_background"
	TaskTypeGenerateScene     = "generate_scene"
	TaskTypeResize            = "resize"
	TaskTypeEnhance           = "enhance"
	TaskTypeTranslateImage    = "translate_image"
	TaskTypePosterGenerate    = "poster_generate"
)

// Status constants.
const (
	StatusPending             = "pending"
	StatusRunning             = "running"
	StatusRetrying            = "retrying"
	StatusSuccess             = "success"
	StatusFailed              = "failed"
	StatusCancelled           = "cancelled"
	StatusSuccessWithWarnings = "success_with_warnings"
	StatusLowQuality          = "low_quality"
	StatusFailedValidation    = "failed_render_validation"
	StatusNeedManualReview    = "need_manual_review"
	StatusSuccessWithReview   = "success_with_review"
	StatusObsolete            = "obsolete"
)

// ImageTask records one AI image processing job (ai_image_tasks in product docs).
type ImageTask struct {
	model.HardDeleteBase
	TaskType        string         `gorm:"size:64;index;not null" json:"taskType"`
	Provider        string         `gorm:"size:64;index;not null" json:"provider"`
	Status          string         `gorm:"size:32;index;not null" json:"status"`
	ProductID       *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	SourceImageID   *uuid.UUID     `gorm:"type:char(36);index" json:"sourceImageId,omitempty"`
	SourceImageURL  string         `gorm:"type:text" json:"sourceImageUrl,omitempty"`
	InputMode       string         `gorm:"size:32" json:"inputMode,omitempty"`
	Prompt          string         `gorm:"type:text" json:"prompt,omitempty"`
	NegativePrompt  string         `gorm:"type:text" json:"negativePrompt,omitempty"`
	OptionsJSON     datatypes.JSON `gorm:"type:jsonb" json:"optionsJson,omitempty"`
	ResultCount     int            `gorm:"default:0" json:"resultCount"`
	Input           datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output          datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	ResultFileID    *uuid.UUID     `gorm:"type:char(36);index" json:"resultFileId,omitempty"`
	ResultURL       string         `gorm:"type:text" json:"resultUrl,omitempty"`
	ErrorMessage    string         `gorm:"type:text" json:"errorMessage,omitempty"`
	RetryCount      int            `gorm:"default:0" json:"retryCount"`
	MaxRetries      int            `gorm:"default:0" json:"maxRetries"`
	NextRetryAt     *time.Time     `json:"nextRetryAt,omitempty"`
	RetryEnqueuedAt *time.Time     `json:"retryEnqueuedAt,omitempty"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	BatchID         *uuid.UUID     `gorm:"type:char(36);index" json:"batchId,omitempty"`
	BatchNo         string         `gorm:"size:64;index" json:"batchNo,omitempty"`
	StartedAt       *time.Time     `json:"startedAt,omitempty"`
	FinishedAt      *time.Time     `json:"finishedAt,omitempty"`
	CompletedAt     *time.Time     `json:"completedAt,omitempty"`
	LockedBy        *string        `gorm:"size:220;index" json:"lockedBy,omitempty"`
	LockedUntil     *time.Time     `gorm:"index" json:"lockedUntil,omitempty"`
	LockVersion     int            `gorm:"default:0;not null" json:"lockVersion"`
}

func (ImageTask) TableName() string { return "image_tasks" }

// ImageTaskItem is one source→result row within a task (ai_image_task_items).
type ImageTaskItem struct {
	model.HardDeleteBase
	TaskID           uuid.UUID      `gorm:"type:char(36);index;not null" json:"taskId"`
	ProductID        *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	SourceImageID    *uuid.UUID     `gorm:"type:char(36);index" json:"sourceImageId,omitempty"`
	SourceImageURL   string         `gorm:"type:text" json:"sourceImageUrl,omitempty"`
	OutputImageURL   string         `gorm:"type:text" json:"outputImageUrl,omitempty"`
	OutputStorageKey string         `gorm:"size:512" json:"outputStorageKey,omitempty"`
	OutputFileID     *uuid.UUID     `gorm:"type:char(36);index" json:"outputFileId,omitempty"`
	ScoreJSON        datatypes.JSON `gorm:"type:jsonb" json:"scoreJson,omitempty"`
	IsSelectedBest   bool           `gorm:"default:false" json:"isSelectedBest"`
	Status           string         `gorm:"size:32;index;not null" json:"status"`
	ErrorMessage     string         `gorm:"type:text" json:"errorMessage,omitempty"`
}

func (ImageTaskItem) TableName() string { return "ai_image_task_items" }

const (
	ItemStatusPending = "pending"
	ItemStatusRunning = "running"
	ItemStatusSuccess = "success"
	ItemStatusFailed  = "failed"
)

func isValidTaskType(t string) bool {
	switch strings.TrimSpace(strings.ToLower(t)) {
	case TaskTypeRemoveBackground,
		TaskTypeReplaceBackground,
		TaskTypeGenerateScene,
		TaskTypeResize,
		TaskTypeEnhance,
		TaskTypeTranslateImage,
		TaskTypePosterGenerate,
		TaskTypeRemoveWatermark,
		TaskTypeRemoveLogo,
		TaskTypeRemoveBadge,
		TaskTypeRemoveQRCode,
		TaskTypeCleanup,
		TaskTypeEnhanceDetail,
		TaskTypeGenerateMarketing,
		TaskTypeGenerateMainImage,
		TaskTypeBatchGenerateMain,
		TaskTypeUpscale,
		TaskTypeScoreImage,
		TaskTypeSelectBestMain,
		TaskTypeTranslateImageText:
		return true
	default:
		return false
	}
}

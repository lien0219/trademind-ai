package imagetask

import (
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
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusRetrying  = "retrying"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// ImageTask records one image processing job.
type ImageTask struct {
	model.HardDeleteBase
	TaskType        string         `gorm:"size:64;index;not null" json:"taskType"`
	Provider        string         `gorm:"size:64;index;not null" json:"provider"`
	Status          string         `gorm:"size:32;index;not null" json:"status"`
	ProductID       *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	SourceImageID   *uuid.UUID     `gorm:"type:char(36);index" json:"sourceImageId,omitempty"`
	SourceImageURL  string         `gorm:"type:text" json:"sourceImageUrl,omitempty"`
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
	StartedAt       *time.Time     `json:"startedAt,omitempty"`
	FinishedAt      *time.Time     `json:"finishedAt,omitempty"`
	LockedBy        *string        `gorm:"size:220;index" json:"lockedBy,omitempty"`
	LockedUntil     *time.Time     `gorm:"index" json:"lockedUntil,omitempty"`
	LockVersion     int            `gorm:"default:0;not null" json:"lockVersion"`
}

func (ImageTask) TableName() string { return "image_tasks" }

func isValidTaskType(t string) bool {
	switch t {
	case TaskTypeRemoveBackground,
		TaskTypeReplaceBackground,
		TaskTypeGenerateScene,
		TaskTypeResize,
		TaskTypeEnhance,
		TaskTypeTranslateImage,
		TaskTypePosterGenerate:
		return true
	default:
		return false
	}
}

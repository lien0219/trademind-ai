package aitask

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Task status values for AI calls.
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// AITask records one AI invocation (audit + observability).
type AITask struct {
	model.HardDeleteBase
	TaskType       string         `gorm:"size:64;index;not null" json:"taskType"`
	Provider       string         `gorm:"size:64" json:"provider"`
	Model          string         `gorm:"size:128" json:"model"`
	PromptCode     string         `gorm:"size:64;index" json:"promptCode"`
	Input          datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output         datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	RawResponse    datatypes.JSON `gorm:"type:jsonb" json:"rawResponse,omitempty"`
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	ErrorMessage   string         `gorm:"type:text" json:"errorMessage,omitempty"`
	TokenInput     int            `gorm:"default:0" json:"tokenInput"`
	TokenOutput    int            `gorm:"default:0" json:"tokenOutput"`
	CostAmount     float64        `gorm:"default:0" json:"costAmount"`
	ProductID      *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	ConversationID *uuid.UUID     `gorm:"type:char(36);index" json:"conversationId,omitempty"`
	CreatedBy      *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	BatchID        *uuid.UUID     `gorm:"type:char(36);index" json:"batchId,omitempty"`
	BatchNo        string         `gorm:"size:64;index" json:"batchNo,omitempty"`
	StartedAt      *time.Time     `json:"startedAt,omitempty"`
	FinishedAt     *time.Time     `json:"finishedAt,omitempty"`
}

func (AITask) TableName() string { return "ai_tasks" }

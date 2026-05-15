package aiprompt

import (
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// AIPrompt is an editable prompt template row.
type AIPrompt struct {
	model.HardDeleteBase
	Code         string         `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	Scene        string         `gorm:"size:64;index" json:"scene"`
	Provider     string         `gorm:"size:64" json:"provider"`
	Model        string         `gorm:"size:128" json:"model"`
	SystemPrompt string         `gorm:"type:text" json:"systemPrompt"`
	UserPrompt   string         `gorm:"type:text" json:"userPrompt"`
	OutputSchema datatypes.JSON `gorm:"type:jsonb" json:"outputSchema,omitempty"`
	Temperature  float64        `gorm:"default:0.7" json:"temperature"`
	MaxTokens    int            `gorm:"default:512" json:"maxTokens"`
	Enabled      bool           `gorm:"default:true;index" json:"enabled"`
}

func (AIPrompt) TableName() string { return "ai_prompts" }

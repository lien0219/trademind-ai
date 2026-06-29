package customerchat

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CustomerConversation is a manual/customer-service conversation (no platform sync in MVP).
type CustomerConversation struct {
	model.Base
	Platform               string         `gorm:"size:64;index;not null" json:"platform"`
	ShopID                 *uuid.UUID     `gorm:"type:char(36);index" json:"shopId,omitempty"`
	ExternalConversationID *string        `gorm:"size:255;index" json:"externalConversationId,omitempty"`
	CustomerName           string         `gorm:"size:255;not null" json:"customerName"`
	CustomerAvatar         string         `gorm:"type:text" json:"customerAvatar,omitempty"`
	CustomerLanguage       string         `gorm:"size:32;default:en;not null" json:"customerLanguage"`
	Status                 string         `gorm:"size:32;index;not null" json:"status"`
	LastMessageAt          *time.Time     `json:"lastMessageAt,omitempty"`
	OrderID                *uuid.UUID     `gorm:"type:char(36);index" json:"orderId,omitempty"`
	RawData                datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	CreatedBy              *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (CustomerConversation) TableName() string { return "customer_conversations" }

// CustomerMessage is one line in a conversation timeline.
type CustomerMessage struct {
	ID                uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	ConversationID    uuid.UUID      `gorm:"type:char(36);index;not null" json:"conversationId"`
	Role              string         `gorm:"size:32;index;not null" json:"role"`
	Content           string         `gorm:"type:text;not null" json:"content"`
	Language          string         `gorm:"size:32;not null" json:"language"`
	MessageType       string         `gorm:"size:32;default:text;not null" json:"messageType"`
	Source            string         `gorm:"size:32;index;not null" json:"source"`
	ExternalMessageID *string        `gorm:"size:255" json:"externalMessageId,omitempty"`
	RawData           datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	CreatedBy         *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	CreatedAt         time.Time      `json:"createdAt"`
}

func (CustomerMessage) TableName() string { return "customer_messages" }

// BeforeCreate assigns UUID when missing.
func (m *CustomerMessage) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&m.ID)
	return nil
}

// CustomerReplySuggestion stores AI-suggested draft replies (human must confirm; never auto-sent externally).
type CustomerReplySuggestion struct {
	model.HardDeleteBase
	ConversationID uuid.UUID      `gorm:"type:char(36);index;not null" json:"conversationId"`
	MessageID      *uuid.UUID     `gorm:"type:char(36);index" json:"messageId,omitempty"`
	AITaskID       *uuid.UUID     `gorm:"type:char(36);index" json:"aiTaskId,omitempty"`
	Provider       string         `gorm:"size:64" json:"provider,omitempty"`
	Model          string         `gorm:"size:128" json:"model,omitempty"`
	PromptCode     string         `gorm:"size:64;index" json:"promptCode,omitempty"`
	SuggestedReply string         `gorm:"type:text" json:"suggestedReply,omitempty"`
	EditedReply    string         `gorm:"type:text" json:"editedReply,omitempty"`
	RejectReason   string         `gorm:"type:text" json:"rejectReason,omitempty"`
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	Language       string         `gorm:"size:32" json:"language,omitempty"`
	Tone           string         `gorm:"size:64" json:"tone,omitempty"`
	Input          datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output         datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	CreatedBy      *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (CustomerReplySuggestion) TableName() string { return "customer_reply_suggestions" }

package customerchat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/gorm"
)

// SuggestionDTO is safe suggestion row for admin (no full prompt).
type SuggestionDTO struct {
	ID             uuid.UUID      `json:"id"`
	ConversationID uuid.UUID      `json:"conversationId"`
	MessageID      *uuid.UUID     `json:"messageId,omitempty"`
	Status         string         `json:"status"`
	SuggestedReply string         `json:"suggestedReply,omitempty"`
	EditedReply    string         `json:"editedReply,omitempty"`
	RejectReason   string         `json:"rejectReason,omitempty"`
	Language       string         `json:"language,omitempty"`
	Tone           string         `json:"tone,omitempty"`
	ContextSummary ContextSummary `json:"contextSummary,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

func suggestionToDTO(row *CustomerReplySuggestion, ctx ContextSummary) *SuggestionDTO {
	if row == nil {
		return nil
	}
	return &SuggestionDTO{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		MessageID:      row.MessageID,
		Status:         row.Status,
		SuggestedReply: row.SuggestedReply,
		EditedReply:    row.EditedReply,
		RejectReason:   row.RejectReason,
		Language:       row.Language,
		Tone:           row.Tone,
		ContextSummary: ctx,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

// ListSuggestions returns AI suggestions for a conversation (newest first).
func (s *Service) ListSuggestions(c *gin.Context, conversationID uuid.UUID) ([]SuggestionDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	var conv CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&conv, "id = ?", conversationID).Error; err != nil {
		return nil, err
	}
	var rows []CustomerReplySuggestion
	if err := s.DB.WithContext(c.Request.Context()).
		Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Limit(20).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	ctxBase := s.buildContextSummary(c, &conv, "")
	out := make([]SuggestionDTO, 0, len(rows))
	for i := range rows {
		d := suggestionToDTO(&rows[i], ctxBase)
		if d != nil {
			out = append(out, *d)
		}
	}
	return out, nil
}

// RejectSuggestionBody POST reject
type RejectSuggestionBody struct {
	Reason string `json:"reason"`
}

// RejectSuggestion marks suggestion rejected with optional reason.
func (s *Service) RejectSuggestion(c *gin.Context, id uuid.UUID, body RejectSuggestionBody, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customerchat: no db")
	}
	var row CustomerReplySuggestion
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return err
	}
	if row.Status == SuggestionAccepted {
		return fmt.Errorf("accepted suggestion cannot be rejected")
	}
	row.Status = SuggestionRejected
	row.RejectReason = truncateRunes(strings.TrimSpace(body.Reason), 500)
	if err := s.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.reply_suggestion.reject",
			Resource:    "customer_reply_suggestion",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     fmt.Sprintf("suggestionId=%s conversationId=%s reasonLen=%d", id.String(), row.ConversationID.String(), len([]rune(row.RejectReason))),
		})
	}
	return nil
}

// ApplySuggestionBody POST apply (alias for send flow preparation).
type ApplySuggestionBody struct {
	FinalReply string `json:"finalReply"`
}

// ApplySuggestion validates edited content before human send (does not auto-send).
func (s *Service) ApplySuggestion(c *gin.Context, id uuid.UUID, body ApplySuggestionBody, adminID *uuid.UUID) error {
	final := strings.TrimSpace(body.FinalReply)
	if final == "" {
		return fmt.Errorf("finalReply is required")
	}
	return s.UpdateSuggestion(c, id, UpdateSuggestionBody{EditedReply: final}, adminID)
}

func parseBoolQuery(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *Service) countOpenFailures(ctx context.Context, conversationID uuid.UUID) int64 {
	if s == nil || s.DB == nil {
		return 0
	}
	var n int64
	_ = s.DB.WithContext(ctx).Model(&CustomerFailureEvent{}).
		Where("conversation_id = ? AND status = ?", conversationID, FailureEventStatusOpen).
		Count(&n).Error
	return n
}

func latestSuggestionStatus(ctx context.Context, db *gorm.DB, conversationID uuid.UUID) string {
	if db == nil {
		return ""
	}
	var row CustomerReplySuggestion
	err := db.WithContext(ctx).Where("conversation_id = ?", conversationID).Order("created_at DESC").First(&row).Error
	if err != nil {
		return ""
	}
	return row.Status
}

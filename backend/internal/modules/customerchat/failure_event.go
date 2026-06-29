package customerchat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

// Customer failure categories (failure task center + dedup key).
const (
	FailureCategoryMessageSyncFailed     = "customer_message_sync_failed"
	FailureCategoryMessageSyncPartial    = "customer_message_sync_partial_success"
	FailureCategoryReplyGenerateFailed   = "customer_reply_generate_failed"
	FailureCategoryReplySendFailed       = "customer_reply_send_failed"
	FailureCategoryReplyPermissionDenied = "customer_reply_permission_denied"
	FailureCategoryPlatformNotAuthorized = "customer_platform_not_authorized"
	FailureCategoryConversationNotFound  = "customer_conversation_not_found"
	FailureEventStatusOpen               = "open"
	FailureEventStatusResolved           = "resolved"
)

// CustomerFailureEvent records customer-domain failures for the unified failure task center.
type CustomerFailureEvent struct {
	model.HardDeleteBase
	ConversationID uuid.UUID  `gorm:"type:char(36);index;not null" json:"conversationId"`
	SuggestionID   *uuid.UUID `gorm:"type:char(36);index" json:"suggestionId,omitempty"`
	SyncTaskID     *uuid.UUID `gorm:"type:char(36);index" json:"syncTaskId,omitempty"`
	Platform       string     `gorm:"size:64;index" json:"platform,omitempty"`
	ShopID         *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	Category       string     `gorm:"size:64;index;not null" json:"category"`
	ErrorMessage   string     `gorm:"type:text" json:"errorMessage,omitempty"`
	Status         string     `gorm:"size:32;index;not null" json:"status"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
}

func (CustomerFailureEvent) TableName() string { return "customer_failure_events" }

func (s *Service) recordFailure(ctx context.Context, ev CustomerFailureEvent) error {
	if s == nil || s.DB == nil || ev.ConversationID == uuid.Nil {
		return fmt.Errorf("customerchat: cannot record failure")
	}
	cat := strings.TrimSpace(ev.Category)
	if cat == "" {
		return fmt.Errorf("customerchat: failure category required")
	}
	ev.Category = cat
	if strings.TrimSpace(ev.Status) == "" {
		ev.Status = FailureEventStatusOpen
	}
	ev.ErrorMessage = truncateRunes(strings.TrimSpace(ev.ErrorMessage), 500)

	// Dedup: one open row per conversation+category (+ optional suggestion).
	tx := s.DB.WithContext(ctx).Model(&CustomerFailureEvent{}).
		Where("conversation_id = ? AND category = ? AND status = ?", ev.ConversationID, cat, FailureEventStatusOpen)
	if ev.SuggestionID != nil {
		tx = tx.Where("suggestion_id = ?", *ev.SuggestionID)
	}
	var existing CustomerFailureEvent
	err := tx.Order("updated_at DESC").First(&existing).Error
	if err == nil {
		return s.DB.WithContext(ctx).Model(&CustomerFailureEvent{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"error_message": ev.ErrorMessage,
			"platform":      ev.Platform,
			"shop_id":       ev.ShopID,
			"updated_at":    time.Now().UTC(),
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	id.Ensure(&ev.ID)
	return s.DB.WithContext(ctx).Create(&ev).Error
}

func (s *Service) resolveFailures(ctx context.Context, conversationID uuid.UUID, category string, suggestionID *uuid.UUID) {
	if s == nil || s.DB == nil || conversationID == uuid.Nil {
		return
	}
	now := time.Now().UTC()
	tx := s.DB.WithContext(ctx).Model(&CustomerFailureEvent{}).
		Where("conversation_id = ? AND status = ?", conversationID, FailureEventStatusOpen)
	if cat := strings.TrimSpace(category); cat != "" {
		tx = tx.Where("category = ?", cat)
	}
	if suggestionID != nil {
		tx = tx.Where("suggestion_id = ?", *suggestionID)
	}
	_ = tx.Updates(map[string]any{
		"status":      FailureEventStatusResolved,
		"resolved_at": &now,
		"updated_at":  now,
	}).Error
}

func classifySendFailure(err error) string {
	if err == nil {
		return FailureCategoryReplySendFailed
	}
	if errors.Is(err, platformp.ErrPlatformCustomerMessagePermissionDenied) {
		return FailureCategoryReplyPermissionDenied
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not authorized") || strings.Contains(msg, "未授权") || strings.Contains(msg, "unauthorized") {
		return FailureCategoryPlatformNotAuthorized
	}
	return FailureCategoryReplySendFailed
}

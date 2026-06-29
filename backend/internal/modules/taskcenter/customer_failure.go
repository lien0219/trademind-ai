package taskcenter

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
)

func (s *Service) listCustomerFailures(ctx context.Context, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	q := s.DB.WithContext(ctx).Model(&customerchat.CustomerFailureEvent{})
	q = q.Where("status = ?", customerchat.FailureEventStatusOpen)
	if !p.IncludeResolved {
		q = q.Where("status = ?", customerchat.FailureEventStatusOpen)
	}
	q = s.applyMarkFilters(q, TaskTypeCustomerFailure, "customer_failure_events.id::text", p)
	if sid := strings.TrimSpace(p.ShopID); sid != "" {
		if u, err := uuid.Parse(sid); err == nil {
			q = q.Where("shop_id = ?", u)
		}
	}
	if pl := strings.TrimSpace(p.Platform); pl != "" {
		q = q.Where("LOWER(platform) = LOWER(?)", pl)
	}
	if lk := likePat(p.Keyword); lk != "" {
		q = q.Where("(COALESCE(error_message,'') ILIKE ? OR category ILIKE ? OR CAST(id AS TEXT) ILIKE ? OR CAST(conversation_id AS TEXT) ILIKE ?)", lk, lk, lk, lk)
	}
	q = s.applyTimeRange(q, p)

	var rows []customerchat.CustomerFailureEvent
	if err := q.Order("updated_at DESC").Limit(fetchLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID.String()
	}
	ms, err := s.fetchMarks(ctx, TaskTypeCustomerFailure, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTaskDTO, 0, len(rows))
	for i := range rows {
		out = append(out, mapCustomerFailureEvent(&rows[i], ms, now))
	}
	return out, nil
}

func mapCustomerFailureEvent(row *customerchat.CustomerFailureEvent, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	norm := NormFailed
	title := truncateRunes(row.Category, 120)
	dto := UnifiedTaskDTO{
		ID:                  row.ID.String(),
		TaskType:            TaskTypeCustomerFailure,
		SourceTable:         SourceTableCustomerFailureEvents,
		SourceID:            row.ID.String(),
		Platform:            strings.TrimSpace(row.Platform),
		Title:               title,
		Status:              row.Status,
		NormalizedStatus:    norm,
		Retryable:           true,
		ErrorMessage:        truncateRunes(row.ErrorMessage, maxErrorMessageLen),
		ErrorCode:           row.Category,
		FailureCategory:     row.Category,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		DetailURL:           customerFailureDetailURL(row),
		RetryAction:         retryActionFor(TaskTypeCustomerFailure),
		RawSummary:          truncateRunes("conversationId="+row.ConversationID.String()+" category="+row.Category, maxRawSummaryLen),
		SortKey:             row.UpdatedAt,
		RelatedResourceType: "customer_conversation",
		RelatedResourceID:   row.ConversationID.String(),
	}
	if row.ShopID != nil {
		dto.ShopID = row.ShopID.String()
	}
	if row.SuggestionID != nil {
		dto.RawSummary = truncateRunes(dto.RawSummary+" suggestionId="+row.SuggestionID.String(), maxRawSummaryLen)
	}
	applyMarks(&dto, TaskTypeCustomerFailure, row.ID.String(), marks)
	_ = now
	return dto
}

func customerFailureDetailURL(row *customerchat.CustomerFailureEvent) string {
	if row == nil {
		return ""
	}
	base := "/customer/conversations/" + row.ConversationID.String()
	if row.SuggestionID != nil {
		return base + "?suggestionId=" + row.SuggestionID.String()
	}
	return base
}

package customerchat

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// applyListFilters augments base query from extended list filters.
func (s *Service) applyListFilters(c *gin.Context, tx *gorm.DB, q ListQuery) *gorm.DB {
	if tx == nil {
		return tx
	}
	ctx := c.Request.Context()
	if q.PendingReply {
		tx = tx.Where("status = ?", StatusPendingReply)
	}
	if q.HasOrder {
		tx = tx.Where("order_id IS NOT NULL")
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		pat := "%" + kw + "%"
		tx = tx.Where("(customer_name ILIKE ? OR CAST(id AS TEXT) ILIKE ? OR CAST(order_id AS TEXT) ILIKE ?)", pat, pat, pat)
	}
	if q.HasAiSuggestion {
		tx = tx.Where(`EXISTS (
			SELECT 1 FROM customer_reply_suggestions s
			WHERE s.conversation_id = customer_conversations.id
			AND s.status IN ?
		)`, []string{SuggestionGenerated, SuggestionEdited})
	}
	if q.SendFailed {
		tx = tx.Where(`EXISTS (
			SELECT 1 FROM customer_failure_events f
			WHERE f.conversation_id = customer_conversations.id
			AND f.status = ? AND f.category IN ?
		)`, FailureEventStatusOpen, []string{FailureCategoryReplySendFailed, FailureCategoryReplyPermissionDenied})
	}
	if q.UpdatedStart != nil {
		tx = tx.Where("updated_at >= ?", *q.UpdatedStart)
	}
	if q.UpdatedEnd != nil {
		tx = tx.Where("updated_at <= ?", *q.UpdatedEnd)
	}
	_ = ctx
	return tx
}

// enrichListItem adds F4 list columns.
func (s *Service) enrichListItem(c *gin.Context, item *ConversationListItem, row CustomerConversation) {
	if s == nil || item == nil {
		return
	}
	item.CustomerNameMasked = maskCustomerName(row.CustomerName)
	item.OrderID = row.OrderID
	item.AiSuggestionStatus = latestSuggestionStatus(c.Request.Context(), s.DB, row.ID)
	item.OpenFailureCount = int(s.countOpenFailures(c.Request.Context(), row.ID))
	if row.OrderID != nil && s.Orders != nil {
		sum, err := s.Orders.ConversationSummary(c, *row.OrderID)
		if err == nil && sum != nil {
			item.OrderNo = sum.OrderNo
			if item.ProductTitle == "" && sum.ItemCount > 0 {
				item.ProductTitle = s.firstOrderProductTitle(c, *row.OrderID)
			}
		}
	}
	if item.AiSuggestionStatus == SuggestionSendFailed || item.OpenFailureCount > 0 {
		item.SendStatus = "failed"
	} else if item.AiSuggestionStatus == SuggestionAccepted {
		item.SendStatus = "sent"
	} else if item.AiSuggestionStatus == SuggestionGenerated || item.AiSuggestionStatus == SuggestionEdited {
		item.SendStatus = "pending_confirm"
	} else {
		item.SendStatus = "none"
	}
}

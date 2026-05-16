package customerchat

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

// Service orchestrates customer chat MVP (manual inbox + AI suggestions).
type Service struct {
	DB        *gorm.DB
	Settings  *settings.Service
	Prompts   *aiprompt.Service
	AITasks   *aitask.Service
	AIGateway *aigate.Gateway
	OpLog     *operationlog.Service
	Orders    *order.Service
}

// --- list ---

// ListQuery binds GET /customer/conversations
type ListQuery struct {
	Page         int
	PageSize     int
	Platform     string
	Status       string
	CustomerName string
	Start        *time.Time
	End          *time.Time
}

// ListResult paginates conversations.
type ListResult struct {
	Items      []ConversationListItem
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// ConversationListItem is one row for ProTable (includes summary fields).
type ConversationListItem struct {
	ID               uuid.UUID  `json:"id"`
	Platform         string     `json:"platform"`
	CustomerName     string     `json:"customerName"`
	CustomerLanguage string     `json:"customerLanguage"`
	Status           string     `json:"status"`
	LastMessageAt    *time.Time `json:"lastMessageAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	MessageCount     int64      `json:"messageCount"`
	LatestMessage    string     `json:"latestMessage,omitempty"`
}

func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}

	tx := s.DB.WithContext(c.Request.Context()).Model(&CustomerConversation{})
	if v := strings.TrimSpace(q.Platform); v != "" {
		tx = tx.Where("platform = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.CustomerName); v != "" {
		tx = tx.Where("customer_name ILIKE ?", "%"+v+"%")
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps

	var rows []CustomerConversation
	if err := tx.Order("COALESCE(last_message_at, created_at) DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &ListResult{Items: []ConversationListItem{}, Total: total, Page: page, PageSize: ps, TotalPages: pagesOf(total, ps)}, nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}

	var counts []struct {
		ConversationID uuid.UUID `gorm:"column:conversation_id"`
		Cnt            int64     `gorm:"column:cnt"`
	}
	if err := s.DB.WithContext(c.Request.Context()).Model(&CustomerMessage{}).
		Select("conversation_id, COUNT(*) as cnt").
		Where("conversation_id IN ?", ids).
		Group("conversation_id").
		Scan(&counts).Error; err != nil {
		return nil, err
	}
	countMap := make(map[uuid.UUID]int64, len(counts))
	for _, ct := range counts {
		countMap[ct.ConversationID] = ct.Cnt
	}

	type latestRow struct {
		ConversationID uuid.UUID `gorm:"column:conversation_id"`
		Content        string    `gorm:"column:content"`
	}
	var latest []latestRow
	if err := s.DB.WithContext(c.Request.Context()).Raw(`
SELECT DISTINCT ON (conversation_id) conversation_id, content
FROM customer_messages
WHERE conversation_id IN ?
ORDER BY conversation_id, created_at DESC
`, ids).Scan(&latest).Error; err != nil {
		return nil, err
	}
	latestMap := make(map[uuid.UUID]string, len(latest))
	for _, l := range latest {
		latestMap[l.ConversationID] = l.Content
	}

	out := make([]ConversationListItem, 0, len(rows))
	for _, r := range rows {
		lm := latestMap[r.ID]
		out = append(out, ConversationListItem{
			ID:               r.ID,
			Platform:         r.Platform,
			CustomerName:     r.CustomerName,
			CustomerLanguage: r.CustomerLanguage,
			Status:           r.Status,
			LastMessageAt:    r.LastMessageAt,
			CreatedAt:        r.CreatedAt,
			UpdatedAt:        r.UpdatedAt,
			MessageCount:     countMap[r.ID],
			LatestMessage:    truncateRunes(lm, 200),
		})
	}

	return &ListResult{
		Items:      out,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}

func pagesOf(total int64, ps int) int {
	if ps < 1 {
		ps = 20
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return pages
}

// --- CRUD conversation ---

// CreateConversationBody POST /customer/conversations
type CreateConversationBody struct {
	Platform         string `json:"platform"`
	CustomerName     string `json:"customerName"`
	CustomerLanguage string `json:"customerLanguage"`
	CustomerAvatar   string `json:"customerAvatar"`
}

func (s *Service) CreateConversation(c *gin.Context, body CreateConversationBody, adminID *uuid.UUID) (*CustomerConversation, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	platform := strings.TrimSpace(body.Platform)
	if platform == "" {
		platform = "manual"
	}
	name := strings.TrimSpace(body.CustomerName)
	if name == "" {
		return nil, fmt.Errorf("customerName is required")
	}
	lang := strings.TrimSpace(body.CustomerLanguage)
	if lang == "" {
		lang = "en"
	}
	row := &CustomerConversation{
		Platform:         platform,
		CustomerName:     name,
		CustomerLanguage: lang,
		CustomerAvatar:   strings.TrimSpace(body.CustomerAvatar),
		Status:           StatusOpen,
		CreatedBy:        adminID,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.conversation.create",
			Resource:    "customer_conversation",
			ResourceID:  row.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("platform=%s conversationId=%s", platform, row.ID.String()),
		})
	}
	return row, nil
}

// ConversationDetailDTO GET /customer/conversations/:id
type ConversationDetailDTO struct {
	ID                     uuid.UUID                       `json:"id"`
	Platform               string                          `json:"platform"`
	ShopID                 *uuid.UUID                      `json:"shopId,omitempty"`
	ExternalConversationID *string                         `json:"externalConversationId,omitempty"`
	CustomerName           string                          `json:"customerName"`
	CustomerAvatar         string                          `json:"customerAvatar,omitempty"`
	CustomerLanguage       string                          `json:"customerLanguage"`
	Status                 string                          `json:"status"`
	LastMessageAt          *time.Time                      `json:"lastMessageAt,omitempty"`
	OrderID                *uuid.UUID                      `json:"orderId,omitempty"`
	OrderSummary           *order.ConversationOrderSummary `json:"orderSummary,omitempty"`
	CreatedBy              *uuid.UUID                      `json:"createdBy,omitempty"`
	CreatedAt              time.Time                       `json:"createdAt"`
	UpdatedAt              time.Time                       `json:"updatedAt"`
}

func convToDTO(r *CustomerConversation, sum *order.ConversationOrderSummary) *ConversationDetailDTO {
	if r == nil {
		return nil
	}
	return &ConversationDetailDTO{
		ID:                     r.ID,
		Platform:               r.Platform,
		ShopID:                 r.ShopID,
		ExternalConversationID: r.ExternalConversationID,
		CustomerName:           r.CustomerName,
		CustomerAvatar:         r.CustomerAvatar,
		CustomerLanguage:       r.CustomerLanguage,
		Status:                 r.Status,
		LastMessageAt:          r.LastMessageAt,
		OrderID:                r.OrderID,
		OrderSummary:           sum,
		CreatedBy:              r.CreatedBy,
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
}

func (s *Service) GetConversation(c *gin.Context, id uuid.UUID) (*ConversationDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	var row CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	var sum *order.ConversationOrderSummary
	if row.OrderID != nil && s.Orders != nil {
		got, err := s.Orders.ConversationSummary(c, *row.OrderID)
		if err == nil && got != nil {
			sum = got
		}
	}
	return convToDTO(&row, sum), nil
}

// UpdateConversationBody PUT
type UpdateConversationBody struct {
	CustomerName     *string `json:"customerName"`
	CustomerLanguage *string `json:"customerLanguage"`
	Status           *string `json:"status"`
	OrderID          *string `json:"orderId"`
}

func (s *Service) UpdateConversation(c *gin.Context, id uuid.UUID, body UpdateConversationBody, adminID *uuid.UUID) (*ConversationDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	var row CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	prevStatus := row.Status
	prevOrderIDStr := uuidToStrPtr(row.OrderID)
	if body.CustomerName != nil {
		v := strings.TrimSpace(*body.CustomerName)
		if v == "" {
			return nil, fmt.Errorf("customerName cannot be empty")
		}
		row.CustomerName = v
	}
	if body.CustomerLanguage != nil {
		v := strings.TrimSpace(*body.CustomerLanguage)
		if v == "" {
			return nil, fmt.Errorf("customerLanguage cannot be empty")
		}
		row.CustomerLanguage = v
	}
	if body.Status != nil {
		st := strings.TrimSpace(*body.Status)
		if !validConversationStatus(st) {
			return nil, fmt.Errorf("invalid status")
		}
		row.Status = st
	}

	if body.OrderID != nil {
		raw := strings.TrimSpace(*body.OrderID)
		if raw == "" {
			row.OrderID = nil
		} else {
			u, err := uuid.Parse(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid orderId")
			}
			if s.Orders == nil || s.Orders.DB == nil {
				return nil, fmt.Errorf("order service unavailable")
			}
			var exists int64
			if err := s.Orders.DB.WithContext(c.Request.Context()).Model(&order.Order{}).Where("id = ?", u).Count(&exists).Error; err != nil {
				return nil, err
			}
			if exists == 0 {
				return nil, fmt.Errorf("order not found")
			}
			row.OrderID = &u
		}
		newStr := uuidToStrPtr(row.OrderID)
		if prevOrderIDStr != newStr && s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "customer.conversation.link_order",
				Resource:    "customer_conversation",
				ResourceID:  row.ID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("conversationId=%s orderId=%s", row.ID.String(), strOrDash(newStr)),
			})
		}
	}

	metaChanged := body.CustomerName != nil || body.CustomerLanguage != nil || body.Status != nil

	if err := s.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		if strings.TrimSpace(row.Status) == StatusClosed && prevStatus != StatusClosed {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "customer.conversation.close",
				Resource:    "customer_conversation",
				ResourceID:  row.ID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("conversationId=%s", row.ID.String()),
			})
		} else if metaChanged {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "customer.conversation.update",
				Resource:    "customer_conversation",
				ResourceID:  row.ID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("conversationId=%s", row.ID.String()),
			})
		}
	}

	var sum *order.ConversationOrderSummary
	if row.OrderID != nil && s.Orders != nil {
		got, err := s.Orders.ConversationSummary(c, *row.OrderID)
		if err == nil && got != nil {
			sum = got
		}
	}
	return convToDTO(&row, sum), nil
}

func uuidToStrPtr(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func strOrDash(id string) string {
	if strings.TrimSpace(id) == "" {
		return "-"
	}
	return id
}

func validConversationStatus(st string) bool {
	switch st {
	case StatusOpen, StatusPendingReply, StatusReplied, StatusClosed:
		return true
	default:
		return false
	}
}

func (s *Service) DeleteConversation(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customerchat: no db")
	}
	var row CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).Delete(&CustomerConversation{}, "id = ?", id).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.conversation.close",
			Resource:    "customer_conversation",
			ResourceID:  row.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("soft_deleted=true conversationId=%s", row.ID.String()),
		})
	}
	return nil
}

// --- messages ---

// CreateMessageBody POST
type CreateMessageBody struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Language string `json:"language"`
	Source   string `json:"source"`
}

func validRole(r string) bool {
	switch strings.TrimSpace(r) {
	case RoleCustomer, RoleAgent, RoleAI:
		return true
	default:
		return false
	}
}

func (s *Service) ListMessages(c *gin.Context, conversationID uuid.UUID) ([]CustomerMessage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	if err := s.DB.WithContext(c.Request.Context()).First(&CustomerConversation{}, "id = ?", conversationID).Error; err != nil {
		return nil, err
	}
	var rows []CustomerMessage
	if err := s.DB.WithContext(c.Request.Context()).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) CreateMessage(c *gin.Context, conversationID uuid.UUID, body CreateMessageBody, adminID *uuid.UUID) (*CustomerMessage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	var conv CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&conv, "id = ?", conversationID).Error; err != nil {
		return nil, err
	}
	if !validRole(body.Role) {
		return nil, fmt.Errorf("invalid role")
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	lang := strings.TrimSpace(body.Language)
	if lang == "" {
		lang = conv.CustomerLanguage
		if lang == "" {
			lang = "en"
		}
	}
	src := strings.TrimSpace(body.Source)
	if src == "" {
		src = SourceManual
	}

	msg := &CustomerMessage{
		ConversationID: conversationID,
		Role:           strings.TrimSpace(body.Role),
		Content:        content,
		Language:       lang,
		Source:         src,
		CreatedBy:      adminID,
	}
	now := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		updates := map[string]any{"last_message_at": &now}
		if msg.Role == RoleCustomer {
			updates["status"] = StatusPendingReply
		}
		if err := tx.Model(&CustomerConversation{}).Where("id = ?", conversationID).Updates(updates).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.message.create",
			Resource:    "customer_conversation",
			ResourceID:  conversationID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("conversationId=%s messageId=%s role=%s contentLen=%d", conversationID.String(), msg.ID.String(), msg.Role, utf8.RuneCountInString(content)),
		})
	}
	return msg, nil
}

// MarkRepliedBody POST mark-replied
type MarkRepliedBody struct {
	Reply string `json:"reply"`
}

func (s *Service) MarkReplied(c *gin.Context, conversationID uuid.UUID, body MarkRepliedBody, adminID *uuid.UUID) (*CustomerMessage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	reply := strings.TrimSpace(body.Reply)
	if reply == "" {
		return nil, fmt.Errorf("reply is required")
	}
	var conv CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&conv, "id = ?", conversationID).Error; err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	msg := &CustomerMessage{
		ConversationID: conversationID,
		Role:           RoleAgent,
		Content:        reply,
		Language:       conv.CustomerLanguage,
		Source:         SourceManual,
		CreatedBy:      adminID,
	}
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		return tx.Model(&CustomerConversation{}).Where("id = ?", conversationID).Updates(map[string]any{
			"status":          StatusReplied,
			"last_message_at": &now,
		}).Error
	}); err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.conversation.replied",
			Resource:    "customer_conversation",
			ResourceID:  conversationID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("conversationId=%s messageId=%s replyLen=%d", conversationID.String(), msg.ID.String(), utf8.RuneCountInString(reply)),
		})
	}
	return msg, nil
}

// UpdateSuggestionBody PUT
type UpdateSuggestionBody struct {
	EditedReply string `json:"editedReply"`
}

func (s *Service) UpdateSuggestion(c *gin.Context, id uuid.UUID, body UpdateSuggestionBody, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customerchat: no db")
	}
	text := strings.TrimSpace(body.EditedReply)
	if text == "" {
		return fmt.Errorf("editedReply is required")
	}
	var row CustomerReplySuggestion
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return err
	}
	row.EditedReply = text
	row.Status = SuggestionEdited
	if err := s.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.reply_suggestion.edit",
			Resource:    "customer_reply_suggestion",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     fmt.Sprintf("suggestionId=%s conversationId=%s", id.String(), row.ConversationID.String()),
		})
	}
	return nil
}

// AcceptSuggestionBody POST accept
type AcceptSuggestionBody struct {
	FinalReply string `json:"finalReply"`
}

func (s *Service) AcceptSuggestion(c *gin.Context, id uuid.UUID, body AcceptSuggestionBody, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customerchat: no db")
	}
	final := strings.TrimSpace(body.FinalReply)
	if final == "" {
		return fmt.Errorf("finalReply is required")
	}
	var su CustomerReplySuggestion
	if err := s.DB.WithContext(c.Request.Context()).First(&su, "id = ?", id).Error; err != nil {
		return err
	}
	var conv CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&conv, "id = ?", su.ConversationID).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		msg := &CustomerMessage{
			ConversationID: su.ConversationID,
			Role:           RoleAgent,
			Content:        final,
			Language:       conv.CustomerLanguage,
			Source:         SourceManual,
			CreatedBy:      adminID,
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		if err := tx.Model(&CustomerReplySuggestion{}).Where("id = ?", id).Update("status", SuggestionAccepted).Error; err != nil {
			return err
		}
		if err := tx.Model(&CustomerConversation{}).Where("id = ?", su.ConversationID).Updates(map[string]any{
			"status":          StatusReplied,
			"last_message_at": &now,
		}).Error; err != nil {
			return err
		}
		if s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "customer.reply_suggestion.accept",
				Resource:    "customer_reply_suggestion",
				ResourceID:  id.String(),
				Status:      "success",
				Message:     fmt.Sprintf("suggestionId=%s conversationId=%s agentMessageId=%s replyLen=%d", id.String(), su.ConversationID.String(), msg.ID.String(), utf8.RuneCountInString(final)),
			})
		}
		return nil
	})
}

func (s *Service) DiscardSuggestion(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customerchat: no db")
	}
	var row CustomerReplySuggestion
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ?", id).Error; err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).Model(&CustomerReplySuggestion{}).Where("id = ?", id).Update("status", SuggestionDiscarded).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.reply_suggestion.discard",
			Resource:    "customer_reply_suggestion",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     fmt.Sprintf("suggestionId=%s conversationId=%s", id.String(), row.ConversationID.String()),
		})
	}
	return nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	rn := []rune(s)
	if len(rn) <= max {
		return s
	}
	return string(rn[:max]) + "…"
}

func (s *Service) providerName(c *gin.Context) string {
	if s == nil || s.Settings == nil {
		return "openai_compatible"
	}
	m, err := s.Settings.PlainByGroup(c.Request.Context(), 0, "ai")
	if err != nil {
		return "openai_compatible"
	}
	v := strings.ToLower(strings.TrimSpace(m["provider"]))
	v = strings.ReplaceAll(v, "-", "_")
	if v == "" {
		return "openai_compatible"
	}
	return v
}

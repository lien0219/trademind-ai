package aitask

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListQuery binds query params for global AI task listing.
type ListQuery struct {
	Page           int
	PageSize       int
	TaskType       string
	Status         string
	Provider       string
	Model          string
	PromptCode     string
	ProductID      *uuid.UUID
	ConversationID *uuid.UUID
	BatchID        *uuid.UUID
	Start          *time.Time
	End            *time.Time
}

// ListResult is a paginated slice of AI tasks (summary columns only).
type ListResult struct {
	Items      []AITask
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// List returns paginated ai_tasks without large JSON columns.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aitask: no db")
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

	tx := s.DB.WithContext(c.Request.Context()).Model(&AITask{}).
		Select("id", "task_type", "provider", "model", "prompt_code", "status", "error_message",
			"token_input", "token_output", "cost_amount", "product_id", "conversation_id", "created_by",
			"batch_id", "batch_no", "started_at", "finished_at", "created_at", "updated_at")

	if v := strings.TrimSpace(q.TaskType); v != "" {
		tx = tx.Where("task_type = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.Provider); v != "" {
		tx = tx.Where("provider = ?", v)
	}
	if v := strings.TrimSpace(q.Model); v != "" {
		tx = tx.Where("model = ?", v)
	}
	if v := strings.TrimSpace(q.PromptCode); v != "" {
		tx = tx.Where("prompt_code = ?", v)
	}
	if q.ProductID != nil {
		tx = tx.Where("product_id = ?", *q.ProductID)
	}
	if q.ConversationID != nil {
		tx = tx.Where("conversation_id = ?", *q.ConversationID)
	}
	if q.BatchID != nil {
		tx = tx.Where("batch_id = ?", *q.BatchID)
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
	var items []AITask
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&items).Error; err != nil {
		return nil, err
	}

	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}

	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
	}, nil
}

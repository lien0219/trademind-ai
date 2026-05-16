package imagetask

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListQuery binds query params for global image task listing.
type ListQuery struct {
	Page      int
	PageSize  int
	TaskType  string
	Status    string
	Provider  string
	ProductID *uuid.UUID
	Start     *time.Time
	End       *time.Time
}

// ListResult is paginated image_tasks (summary columns only).
type ListResult struct {
	Items      []ImageTask
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// List returns paginated image_tasks without large JSON columns.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("imagetask: no db")
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

	tx := s.DB.WithContext(c.Request.Context()).Model(&ImageTask{}).
		Select("id", "task_type", "provider", "status", "product_id", "source_image_id", "source_image_url",
			"result_file_id", "result_url", "error_message", "created_by", "started_at", "finished_at", "created_at", "updated_at")

	if v := strings.TrimSpace(q.TaskType); v != "" {
		tx = tx.Where("task_type = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.Provider); v != "" {
		tx = tx.Where("provider = ?", v)
	}
	if q.ProductID != nil {
		tx = tx.Where("product_id = ?", *q.ProductID)
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
	var items []ImageTask
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

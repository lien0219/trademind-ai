package operationlog

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

// WriteOpts is a single audit row to append.
type WriteOpts struct {
	AdminUserID *uuid.UUID
	Username    string
	Action      string
	Resource    string
	ResourceID  string
	Status      string
	Message     string
}

// Service persists operation logs.
type Service struct {
	DB *gorm.DB
}

// Write inserts one log row from the HTTP context plus overrides in opts.
func (s *Service) Write(c *gin.Context, opts WriteOpts) error {
	if s == nil || s.DB == nil || c == nil {
		return nil
	}
	reqID, _ := c.Get(ctxkey.TraceID)
	rid, _ := reqID.(string)

	adminID := opts.AdminUserID
	if adminID == nil {
		if idStr, ok := c.Get(ctxkey.AdminID); ok {
			if sub, ok := idStr.(string); ok {
				if u, err := uuid.Parse(sub); err == nil {
					adminID = &u
				}
			}
		}
	}
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		if u, ok := c.Get(ctxkey.AdminUsername); ok {
			username, _ = u.(string)
			username = strings.TrimSpace(username)
		}
	}

	path := c.Request.URL.Path
	if fp := c.FullPath(); fp != "" {
		path = fp
	}

	row := &OperationLog{
		AdminUserID: adminID,
		Username:    username,
		Action:      strings.TrimSpace(opts.Action),
		Resource:    strings.TrimSpace(opts.Resource),
		ResourceID:  strings.TrimSpace(opts.ResourceID),
		Method:      c.Request.Method,
		Path:        path,
		IP:          c.ClientIP(),
		UserAgent:   truncateRunes(c.Request.UserAgent(), 512),
		RequestID:   rid,
		Status:      strings.TrimSpace(opts.Status),
		Message:     truncateRunes(opts.Message, 2000),
	}
	return s.DB.WithContext(c.Request.Context()).Create(row).Error
}

// WriteBackground inserts one log row without an HTTP request (workers, cron).
func (s *Service) WriteBackground(ctx context.Context, opts WriteOpts) error {
	if s == nil || s.DB == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	adminID := opts.AdminUserID
	username := strings.TrimSpace(opts.Username)

	row := &OperationLog{
		AdminUserID: adminID,
		Username:    username,
		Action:      strings.TrimSpace(opts.Action),
		Resource:    strings.TrimSpace(opts.Resource),
		ResourceID:  strings.TrimSpace(opts.ResourceID),
		Method:      "INTERNAL",
		Path:        "/internal/worker",
		RequestID:   "",
		Status:      strings.TrimSpace(opts.Status),
		Message:     truncateRunes(opts.Message, 2000),
	}
	return s.DB.WithContext(ctx).Create(row).Error
}

// ListQuery binds query params for listing operation logs.
type ListQuery struct {
	Page     int
	PageSize int
	Action   string
	Username string
	Resource string
	Start    *time.Time
	End      *time.Time
}

// ListResult is a paginated slice of logs.
type ListResult struct {
	Items      []OperationLog
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// List returns a paginated list with optional filters.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("operationlog: no db")
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

	tx := s.DB.WithContext(c.Request.Context()).Model(&OperationLog{})
	if v := strings.TrimSpace(q.Action); v != "" {
		tx = tx.Where("action = ?", v)
	}
	if v := strings.TrimSpace(q.Username); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(username) LIKE ?", pat)
	}
	if v := strings.TrimSpace(q.Resource); v != "" {
		tx = tx.Where("resource = ?", v)
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
	var items []OperationLog
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

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}

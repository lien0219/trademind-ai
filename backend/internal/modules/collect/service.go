package collect

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// Service orchestrates collect tasks and persists results via product drafts.
type Service struct {
	DB       *gorm.DB
	Products *product.Service
	OpLog    *operationlog.Service
	Client   *CollectorClient
}

func clampCollectPage(page, ps int) (int, int) {
	if page < 1 {
		page = 1
	}
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	return page, ps
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

type normalizedProduct struct {
	Source            string            `json:"source"`
	SourceURL         string            `json:"sourceUrl"`
	Title             string            `json:"title"`
	Currency          string            `json:"currency"`
	MainImages        []string          `json:"mainImages"`
	DescriptionImages []string          `json:"descriptionImages"`
	Attributes        json.RawMessage   `json:"attributes"`
	SKUs              []json.RawMessage `json:"skus"`
	Raw               json.RawMessage   `json:"raw"`
}

func parseNormalized(b json.RawMessage) (*normalizedProduct, error) {
	var n normalizedProduct
	if err := json.Unmarshal(b, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (n *normalizedProduct) importParams(fullJSON json.RawMessage) product.ImportDraftParams {
	if n == nil {
		return product.ImportDraftParams{FullNormalizedJSON: fullJSON}
	}
	var skus []product.ImportSKUParams
	for _, raw := range n.SKUs {
		line, err := product.BuildImportSKU(raw)
		if err != nil {
			continue
		}
		skus = append(skus, line)
	}
	return product.ImportDraftParams{
		Source:             strings.TrimSpace(n.Source),
		SourceURL:          strings.TrimSpace(n.SourceURL),
		Title:              strings.TrimSpace(n.Title),
		Currency:           strings.TrimSpace(n.Currency),
		MainImages:         n.MainImages,
		DescriptionImages:  n.DescriptionImages,
		SKUs:               skus,
		FullNormalizedJSON: fullJSON,
	}
}

func (s *Service) failTask(c *gin.Context, taskID uuid.UUID, msg string) {
	if s == nil || s.DB == nil {
		return
	}
	msg = truncateRunes(strings.TrimSpace(msg), 8000)
	fin := time.Now().UTC()
	_ = s.DB.WithContext(c.Request.Context()).Model(&CollectTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":        StatusFailed,
			"error_message": msg,
			"finished_at":   &fin,
			"updated_at":    fin,
		}).Error

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "collect.task.failed",
			Resource:   "collect_task",
			ResourceID: taskID.String(),
			Status:     "failed",
			Message:    truncateRunes(msg, 2000),
		})
	}
}

func (s *Service) runPipeline(c *gin.Context, taskID uuid.UUID) {
	if s == nil || s.DB == nil || s.Client == nil || s.Products == nil {
		return
	}
	ctx := c.Request.Context()

	var task CollectTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return
	}

	started := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":        StatusRunning,
			"started_at":    &started,
			"error_message": "",
			"finished_at":   nil,
			"updated_at":    started,
		}).Error; err != nil {
		s.failTask(c, taskID, err.Error())
		return
	}

	outcome, err := s.Client.Collect(ctx, task.Source, task.SourceURL)
	if err != nil {
		msg := err.Error()
		var rej *CollectorRejectedError
		if errors.As(err, &rej) && rej != nil && strings.TrimSpace(rej.Message) != "" {
			msg = rej.Message
		}
		s.failTask(c, taskID, msg)
		return
	}

	norm, err := parseNormalized(outcome.ProductJSON)
	if err != nil {
		s.failTask(c, taskID, fmt.Sprintf("parse normalized product: %v", err))
		return
	}

	params := norm.importParams(outcome.ProductJSON)
	created, err := s.Products.ImportDraft(c, task.CreatedBy, params)
	if err != nil {
		s.failTask(c, taskID, err.Error())
		return
	}

	fin := time.Now().UTC()
	rawJSON := datatypes.JSON(outcome.ProductJSON)
	pid := created.ID
	if err := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":            StatusSuccess,
			"result_product_id": pid,
			"raw_result":        rawJSON,
			"error_message":     "",
			"finished_at":       &fin,
			"updated_at":        fin,
		}).Error; err != nil {
		s.failTask(c, taskID, err.Error())
		return
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "collect.task.success",
			Resource:   "collect_task",
			ResourceID: taskID.String(),
			Status:     "success",
			Message:    fmt.Sprintf("product_id=%s", pid.String()),
		})
	}
}

// CreateAndRun persists a task and runs the collector synchronously.
func (s *Service) CreateAndRun(c *gin.Context, body CreateTaskBody, adminID *uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	source := strings.TrimSpace(body.Source)
	url := strings.TrimSpace(body.URL)
	if source == "" || url == "" {
		return zero, fmt.Errorf("source and url are required")
	}

	task := &CollectTask{
		Source:    source,
		SourceURL: url,
		Status:    StatusPending,
		CreatedBy: adminID,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(task).Error; err != nil {
		return zero, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "collect.task.create",
			Resource:   "collect_task",
			ResourceID: task.ID.String(),
			Status:     "success",
			Message:    "task created",
		})
	}

	s.runPipeline(c, task.ID)
	return s.GetDTO(c, task.ID)
}

// Retry reruns a failed task.
func (s *Service) Retry(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}

	var task CollectTask
	if err := s.DB.WithContext(c.Request.Context()).First(&task, "id = ?", id).Error; err != nil {
		return zero, err
	}
	if task.Status != StatusFailed {
		return zero, fmt.Errorf("only failed tasks can be retried")
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.task.retry",
			Resource:    "collect_task",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     "retry started",
		})
	}

	retryAt := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Model(&CollectTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":            StatusRetrying,
			"error_message":     "",
			"finished_at":       nil,
			"result_product_id": nil,
			"raw_result":        datatypes.JSON(nil),
			"updated_at":        retryAt,
		}).Error; err != nil {
		return zero, err
	}

	s.runPipeline(c, id)
	return s.GetDTO(c, id)
}

// GetDTO returns one task by id.
func (s *Service) GetDTO(c *gin.Context, id uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	var t CollectTask
	if err := s.DB.WithContext(c.Request.Context()).First(&t, "id = ?", id).Error; err != nil {
		return zero, err
	}
	return taskToDTO(&t), nil
}

// List paginates tasks with filters.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collect: no db")
	}
	page, ps := clampCollectPage(q.Page, q.PageSize)

	tx := s.DB.WithContext(c.Request.Context()).Model(&CollectTask{})
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.Source); v != "" {
		tx = tx.Where("source = ?", v)
	}
	if v := strings.TrimSpace(q.Keyword); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(source_url) LIKE ?", pat)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * ps
	var rows []CollectTask
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]TaskDTO, 0, len(rows))
	for i := range rows {
		items = append(items, taskToDTO(&rows[i]))
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

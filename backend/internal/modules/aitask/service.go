package aitask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service persists ai_tasks.
type Service struct {
	DB *gorm.DB
}

// Create inserts a new task row (defaults status running and started_at).
func (s *Service) Create(ctx context.Context, row *AITask) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("aitask: no db")
	}
	if row == nil {
		return fmt.Errorf("aitask: nil row")
	}
	if row.Status == "" {
		row.Status = StatusRunning
	}
	if row.StartedAt == nil {
		now := time.Now().UTC()
		row.StartedAt = &now
	}
	return s.DB.WithContext(ctx).Create(row).Error
}

// MarkSuccess updates a task with output and token usage.
func (s *Service) MarkSuccess(ctx context.Context, id uuid.UUID, output json.RawMessage, raw json.RawMessage, inTok, outTok int, model string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("aitask: no db")
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":        StatusSuccess,
		"output":        datatypes.JSON(output),
		"raw_response":  datatypes.JSON(raw),
		"token_input":   inTok,
		"token_output":  outTok,
		"finished_at":   &now,
		"error_message": "",
	}
	if strings.TrimSpace(model) != "" {
		updates["model"] = strings.TrimSpace(model)
	}
	return s.DB.WithContext(ctx).Model(&AITask{}).Where("id = ?", id).Updates(updates).Error
}

// MarkFailed records failure.
func (s *Service) MarkFailed(ctx context.Context, id uuid.UUID, msg string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("aitask: no db")
	}
	now := time.Now().UTC()
	return s.DB.WithContext(ctx).Model(&AITask{}).Where("id = ?", id).Updates(map[string]any{
		"status":        StatusFailed,
		"error_message": msg,
		"finished_at":   &now,
	}).Error
}

// GetByID loads one task.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*AITask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aitask: no db")
	}
	var row AITask
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ListRecentForProduct returns latest tasks for a product (newest first).
func (s *Service) ListRecentForProduct(ctx context.Context, productID uuid.UUID, limit int) ([]AITask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aitask: no db")
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	var rows []AITask
	if err := s.DB.WithContext(ctx).
		Select("id", "task_type", "provider", "model", "prompt_code", "status", "error_message", "token_input", "token_output", "cost_amount", "product_id", "created_by", "started_at", "finished_at", "created_at", "updated_at").
		Where("product_id = ?", productID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

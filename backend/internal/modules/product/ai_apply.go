package product

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

func (s *Service) applyAIContent(c *gin.Context, p *Product, fieldType string, value string, taskID uuid.UUID, expectedUpdatedAt string, sourceSnapshotHash string, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("product: no db")
	}
	if p == nil {
		return fmt.Errorf("product is required")
	}
	expectedAt, err := parseExpectedUpdatedAt(expectedUpdatedAt)
	if err != nil {
		return err
	}
	taskHash := ""
	if s.AITasks != nil {
		tk, err := s.AITasks.GetByID(c.Request.Context(), taskID)
		if err != nil {
			return err
		}
		if tk.ProductID == nil || *tk.ProductID != p.ID {
			return fmt.Errorf("task does not belong to this product")
		}
		taskHash = aiTaskSourceSnapshotHash(tk.Input)
	}
	if strings.TrimSpace(sourceSnapshotHash) == "" {
		sourceSnapshotHash = taskHash
	}
	if expectedAt != nil && !sameSecondOrAfter(*expectedAt, p.UpdatedAt) {
		return fmt.Errorf("content conflict: product was updated after AI result was generated")
	}
	if sourceSnapshotHash != "" && sourceSnapshotHash != currentSourceHashForField(p, fieldType) {
		return fmt.Errorf("content conflict: source content changed after AI result was generated")
	}

	now := time.Now().UTC()
	prev := currentAIValueForField(p, fieldType)
	app := &ProductAIContentApplication{
		ProductID:          p.ID,
		FieldType:          fieldType,
		AITaskID:           &taskID,
		PreviousValue:      prev,
		AppliedValue:       value,
		SourceSnapshotHash: strings.TrimSpace(sourceSnapshotHash),
		ExpectedUpdatedAt:  expectedAt,
		AppliedBy:          adminID,
		AppliedAt:          now,
		Status:             AIContentApplyStatusApplied,
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(app).Error; err != nil {
			return err
		}
		updates := map[string]any{"updated_at": now}
		switch fieldType {
		case AIContentFieldTitle:
			updates["ai_title"] = value
		case AIContentFieldDescription:
			updates["ai_description"] = value
		default:
			return fmt.Errorf("unsupported ai content field")
		}
		return tx.Model(&Product{}).Where("id = ?", p.ID).Updates(updates).Error
	})
}

// UndoAIContent restores the latest safely restorable AI-applied field value.
func (s *Service) UndoAIContent(c *gin.Context, productID uuid.UUID, fieldType string, body UndoAIContentBody, adminID *uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	var p Product
	if err := s.DB.WithContext(c.Request.Context()).First(&p, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	var app ProductAIContentApplication
	q := s.DB.WithContext(c.Request.Context()).
		Where("product_id = ? AND field_type = ? AND status = ?", productID, fieldType, AIContentApplyStatusApplied)
	if rawID := strings.TrimSpace(body.ApplicationID); rawID != "" {
		aid, err := uuid.Parse(rawID)
		if err != nil {
			return nil, fmt.Errorf("invalid applicationId")
		}
		q = q.Where("id = ?", aid)
	}
	if err := q.Order("applied_at DESC, created_at DESC").First(&app).Error; err != nil {
		return nil, err
	}
	if cur := currentAIValueForField(&p, fieldType); cur != app.AppliedValue {
		return nil, fmt.Errorf("content conflict: AI field changed after application")
	}
	expectedAt, err := parseExpectedUpdatedAt(body.ExpectedUpdatedAt)
	if err != nil {
		return nil, err
	}
	if expectedAt != nil && !sameSecondOrAfter(*expectedAt, p.UpdatedAt) {
		return nil, fmt.Errorf("content conflict: product was updated after page loaded")
	}

	now := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"updated_at": now}
		switch fieldType {
		case AIContentFieldTitle:
			updates["ai_title"] = app.PreviousValue
		case AIContentFieldDescription:
			updates["ai_description"] = app.PreviousValue
		default:
			return fmt.Errorf("unsupported ai content field")
		}
		if err := tx.Model(&Product{}).Where("id = ?", productID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Model(&ProductAIContentApplication{}).Where("id = ?", app.ID).Updates(map[string]any{
			"status":    AIContentApplyStatusUndone,
			"undone_by": adminID,
			"undone_at": &now,
		}).Error
	}); err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		action := "product.ai_title.undo"
		if fieldType == AIContentFieldDescription {
			action = "product.ai_description.undo"
		}
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      action,
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("applicationId=%s", app.ID.String()),
		})
	}
	return s.Get(c, productID)
}

func currentAIValueForField(p *Product, fieldType string) string {
	if p == nil {
		return ""
	}
	switch fieldType {
	case AIContentFieldTitle:
		return strings.TrimSpace(p.AITitle)
	case AIContentFieldDescription:
		return strings.TrimSpace(p.AIDescription)
	default:
		return ""
	}
}

func currentSourceHashForField(p *Product, fieldType string) string {
	if p == nil {
		return ""
	}
	switch fieldType {
	case AIContentFieldTitle:
		return productContentHash(productPromptTitle(p))
	case AIContentFieldDescription:
		return productContentHash(p.Description)
	default:
		return ""
	}
}

func aiTaskSourceSnapshotHash(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m["sourceSnapshotHash"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func sameSecondOrAfter(expected time.Time, current time.Time) bool {
	// Databases may round timestamps. Treat sub-second deltas as the same page version.
	return !current.UTC().After(expected.UTC().Add(time.Second))
}

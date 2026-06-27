package product

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
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
	now := time.Now().UTC()
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var current Product
		if err := tx.First(&current, "id = ?", p.ID).Error; err != nil {
			return err
		}
		taskHash, err := s.validateAITaskForApply(c.Request.Context(), taskID, current.ID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(sourceSnapshotHash) == "" {
			sourceSnapshotHash = taskHash
		}
		if expectedAt != nil && !sameSecondOrAfter(*expectedAt, current.UpdatedAt) {
			return fmt.Errorf("content conflict: product was updated after AI result was generated")
		}
		if sourceSnapshotHash != "" && sourceSnapshotHash != currentSourceHashForField(&current, fieldType) {
			return fmt.Errorf("content conflict: source content changed after AI result was generated")
		}
		prev := currentAIValueForField(&current, fieldType)
		app := &ProductAIContentApplication{
			ProductID:          current.ID,
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
		res := tx.Model(&Product{}).Where("id = ? AND updated_at = ?", current.ID, current.UpdatedAt).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("content conflict: product changed while applying AI result")
		}
		return nil
	})
}

// UndoAIContent restores the latest safely restorable AI-applied field value.
func (s *Service) UndoAIContent(c *gin.Context, productID uuid.UUID, fieldType string, body UndoAIContentBody, adminID *uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	expectedAt, err := parseExpectedUpdatedAt(body.ExpectedUpdatedAt)
	if err != nil {
		return nil, err
	}
	if rawID := strings.TrimSpace(body.ApplicationID); rawID != "" {
		aid, err := uuid.Parse(rawID)
		if err != nil {
			return nil, fmt.Errorf("invalid applicationId")
		}
		body.ApplicationID = aid.String()
	}

	now := time.Now().UTC()
	var undoneApplicationID uuid.UUID
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var current Product
		if err := tx.First(&current, "id = ?", productID).Error; err != nil {
			return err
		}
		var app ProductAIContentApplication
		q := tx.Where("product_id = ? AND field_type = ? AND status = ?", productID, fieldType, AIContentApplyStatusApplied)
		if rawID := strings.TrimSpace(body.ApplicationID); rawID != "" {
			q = q.Where("id = ?", rawID)
		}
		if err := q.Order("applied_at DESC, created_at DESC").First(&app).Error; err != nil {
			return err
		}
		if cur := currentAIValueForField(&current, fieldType); cur != app.AppliedValue {
			return fmt.Errorf("content conflict: AI field changed after application")
		}
		if expectedAt != nil && !sameSecondOrAfter(*expectedAt, current.UpdatedAt) {
			return fmt.Errorf("content conflict: product was updated after page loaded")
		}
		updates := map[string]any{"updated_at": now}
		switch fieldType {
		case AIContentFieldTitle:
			updates["ai_title"] = app.PreviousValue
		case AIContentFieldDescription:
			updates["ai_description"] = app.PreviousValue
		default:
			return fmt.Errorf("unsupported ai content field")
		}
		res := tx.Model(&Product{}).Where("id = ? AND updated_at = ?", productID, current.UpdatedAt).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("content conflict: product changed while undoing AI result")
		}
		res = tx.Model(&ProductAIContentApplication{}).Where("id = ? AND status = ?", app.ID, AIContentApplyStatusApplied).Updates(map[string]any{
			"status":    AIContentApplyStatusUndone,
			"undone_by": adminID,
			"undone_at": &now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("content conflict: AI application was already undone")
		}
		undoneApplicationID = app.ID
		return nil
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
			Message:     fmt.Sprintf("applicationId=%s", undoneApplicationID.String()),
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

func (s *Service) validateAITaskForApply(ctx context.Context, taskID uuid.UUID, productID uuid.UUID) (string, error) {
	if s == nil || s.AITasks == nil {
		return "", nil
	}
	tk, err := s.AITasks.GetByID(ctx, taskID)
	if err != nil {
		return "", err
	}
	if tk.ProductID == nil || *tk.ProductID != productID {
		return "", fmt.Errorf("task does not belong to this product")
	}
	if !strings.EqualFold(strings.TrimSpace(tk.Status), aitask.StatusSuccess) {
		return "", fmt.Errorf("AI result is not ready to apply")
	}
	return aiTaskSourceSnapshotHash(tk.Input), nil
}

func sameSecondOrAfter(expected time.Time, current time.Time) bool {
	// Databases may round timestamps. Treat sub-second deltas as the same page version.
	return !current.UTC().After(expected.UTC().Add(time.Second))
}

package product

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

const (
	CodeAIApplyReconciliationConflict = "AI_APPLY_RECONCILIATION_CONFLICT"
)

// ReconcileAIApply repairs idempotency when business tx committed but Complete did not run.
func (s *Service) ReconcileAIApply(ctx context.Context, productID uuid.UUID, fieldType, taskID, sourceSnapshotHash string, idemRecordID uuid.UUID, idemOwner string) error {
	if s == nil || s.DB == nil || s.Idempotency == nil || idemRecordID == uuid.Nil {
		return nil
	}
	fieldType = strings.TrimSpace(fieldType)
	taskID = strings.TrimSpace(taskID)
	if fieldType == "" || taskID == "" {
		return fmt.Errorf("reconcile: fieldType and taskID required")
	}
	tid, err := uuid.Parse(taskID)
	if err != nil {
		return err
	}

	var app ProductAIContentApplication
	query := s.DB.WithContext(ctx)
	if tc := security.FromContext(ctx); tc != nil {
		if tc.TenantID < 0 {
			return fmt.Errorf("tenant context missing")
		}
		query = query.Where("product_id IN (SELECT id FROM products WHERE tenant_id = ?)", tc.TenantID)
	}
	err = query.
		Where("product_id = ? AND field_type = ? AND ai_task_id = ? AND status = ?",
			productID, fieldType, tid, AIContentApplyStatusApplied).
		Order("applied_at DESC").First(&app).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	var prod Product
	prodQuery := s.DB.WithContext(ctx)
	if tc := security.FromContext(ctx); tc != nil {
		prodQuery = prodQuery.Where("tenant_id = ?", tc.TenantID)
	}
	if err := prodQuery.First(&prod, "id = ?", productID).Error; err != nil {
		return err
	}
	current := currentAIValueForField(&prod, fieldType)
	if strings.TrimSpace(current) != strings.TrimSpace(app.AppliedValue) {
		_ = s.Idempotency.Fail(ctx, idemRecordID, idemOwner, CodeAIApplyReconciliationConflict, false)
		return fmt.Errorf("%s: product field diverged from apply record", CodeAIApplyReconciliationConflict)
	}
	if sourceSnapshotHash != "" && sourceSnapshotHash != strings.TrimSpace(app.SourceSnapshotHash) {
		_ = s.Idempotency.Fail(ctx, idemRecordID, idemOwner, CodeAIApplyReconciliationConflict, false)
		return fmt.Errorf("%s: source snapshot mismatch", CodeAIApplyReconciliationConflict)
	}

	summary, _ := json.Marshal(map[string]string{
		"applicationId": app.ID.String(),
		"reconciled":    "true",
	})
	return s.Idempotency.Complete(ctx, idemRecordID, idemOwner, idempotency.CompleteResult{
		ResponseCode:    "AI_APPLY_RECONCILED",
		ResponseSummary: string(summary),
		ResourceType:    "product_ai_content_application",
		ResourceID:      app.ID.String(),
	})
}

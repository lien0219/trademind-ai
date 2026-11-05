package aiproductimage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// ReconcileImageApply repairs idempotency when image apply committed but Complete did not run.
func (s *Service) ReconcileImageApply(ctx context.Context, job *imageApplyAcquire, item *AIProductImageItem, slot string, tenantIDs ...int64) (bool, error) {
	if s == nil || s.DB == nil || s.Idempotency == nil || job == nil || item == nil || item.ImageTaskID == nil {
		return false, nil
	}

	var app product.ProductImageApplication
	q := s.DB.WithContext(ctx)
	if len(tenantIDs) > 0 {
		q = scopeApplicationQuery(q, tenantIDs[0])
	}
	err := q.Where("product_id = ? AND image_task_id = ? AND status = ?", item.ProductID, item.ImageTaskID, product.ImageApplyStatusApplied).
		Order("applied_at DESC").First(&app).Error
	if err != nil {
		return false, nil
	}

	summary, _ := json.Marshal(map[string]string{
		"applicationId": app.ID.String(),
		"reconciled":    "true",
		"slot":          strings.TrimSpace(slot),
	})
	if err := s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "AI_IMAGE_APPLY_RECONCILED",
		ResponseSummary: string(summary),
		ResourceType:    "product_image_application",
		ResourceID:      app.ID.String(),
	}); err != nil {
		return false, err
	}
	q = s.DB.WithContext(ctx).Model(&AIProductImageItem{})
	if len(tenantIDs) > 0 {
		q = scopeItemQuery(q, tenantIDs[0])
	}
	_ = q.Where("id = ?", item.ID).Updates(map[string]any{"status": ItemApplied}).Error
	return true, nil
}

// reconcileImageApplyConflict marks reconciliation conflict when manual edits diverge.
func reconcileImageApplyConflict(ctx context.Context, idem *idempotency.Service, recordID uuid.UUID, owner string) error {
	if idem == nil || recordID == uuid.Nil {
		return nil
	}
	_ = idem.Fail(ctx, recordID, owner, product.CodeAIApplyReconciliationConflict, false)
	return fmt.Errorf("%s", product.CodeAIApplyReconciliationConflict)
}

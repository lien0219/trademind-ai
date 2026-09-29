package inventory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"gorm.io/gorm"
)

const errInventoryDeductInProgress = "INVENTORY_DEDUCT_IN_PROGRESS"

type deductAcquire struct {
	RecordID uuid.UUID
	Owner    string
	Key      string
}

// deductCompletion is kept separate from the inventory transaction. The
// idempotency record uses its own transaction, so completing it before the
// ledger commit could leave a successful replay record without a stock fact.
type deductCompletion struct {
	Job   *deductAcquire
	LogID uuid.UUID
}

func deductRequestHash(orderID, orderItemID, skuID uuid.UUID, qty int, reason string) string {
	payload, _ := json.Marshal(map[string]any{
		"orderId":     orderID.String(),
		"orderItemId": orderItemID.String(),
		"skuId":       skuID.String(),
		"qty":         qty,
		"reason":      reason,
	})
	return idempotency.HashRequest(payload)
}

func deductOwner(opts OrderInventoryOptions) string {
	if opts.CreatedBy != nil && *opts.CreatedBy != uuid.Nil {
		return opts.CreatedBy.String()
	}
	return "inventory-deduct"
}

func (s *Service) acquireDeductLine(ctx context.Context, orderID uuid.UUID, orderItemID, skuID uuid.UUID, qty int, reason string, opts OrderInventoryOptions) (*deductAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil {
		return nil, nil, nil
	}
	key := idempotency.InventoryDeduct(orderID.String(), orderItemID.String(), skuID.String())
	owner := deductOwner(opts)
	hash := deductRequestHash(orderID, orderItemID, skuID, qty, reason)
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeInventory, key, hash, owner, idempotency.DefaultLease)
	decision, rec, _ := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return nil, res, nil
	case idempotency.DecisionInProgress:
		return nil, res, fmt.Errorf("%s", errInventoryDeductInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, res, fmt.Errorf("INVENTORY_DEDUCT_KEY_CONFLICT")
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
		if rec == nil {
			return nil, res, fmt.Errorf("idempotency: missing record")
		}
		return &deductAcquire{RecordID: rec.ID, Owner: owner, Key: key}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completeDeductLine(ctx context.Context, job *deductAcquire, chgID uuid.UUID) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	summary, _ := json.Marshal(map[string]string{"changeLogId": chgID.String()})
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "INVENTORY_DEDUCT_SUCCESS",
		ResponseSummary: string(summary),
		ResourceType:    "inventory_change_log",
		ResourceID:      chgID.String(),
	})
}

func (s *Service) failDeductLine(ctx context.Context, job *deductAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func (s *Service) resolveExistingDeductOutcome(ctx context.Context, tx *gorm.DB, orderItemID, skuID uuid.UUID) (deductLineOutcome, error) {
	out := deductLineOutcome{skipped: true}
	if tx == nil {
		return out, fmt.Errorf("inventory: no db tx")
	}
	var hitSuccess int64
	if err := tx.Model(&OrderInventoryEffect{}).
		Where("order_item_id = ? AND product_sku_id = ? AND effect_type = ? AND status = ?", orderItemID, skuID, EffectTypeDeduct, InventoryEffectSuccess).
		Count(&hitSuccess).Error; err != nil {
		return out, err
	}
	if hitSuccess > 0 {
		out.synced = true
		out.skipped = false
		return out, nil
	}
	var hitFailed int64
	if err := tx.Model(&OrderInventoryEffect{}).
		Where("order_item_id = ? AND product_sku_id = ? AND effect_type = ? AND status = ?", orderItemID, skuID, EffectTypeDeduct, InventoryEffectFailed).
		Count(&hitFailed).Error; err != nil {
		return out, err
	}
	if hitFailed > 0 {
		out.failedInsufficient = true
		out.skipped = false
		return out, nil
	}
	return out, nil
}

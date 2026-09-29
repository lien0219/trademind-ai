package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) replayCompletedFulfillmentWave(c *gin.Context, tenantID int64, principal *adminperm.Principal, waveID uuid.UUID, record *idempotency.Record) (*CompleteFulfillmentWaveResult, error) {
	wave, err := s.GetFulfillmentWave(c.Request.Context(), tenantID, principal, waveID)
	if err != nil {
		return nil, err
	}
	out := &CompleteFulfillmentWaveResult{Wave: wave}
	if record != nil && strings.TrimSpace(record.ResponseSummary) != "" {
		_ = json.Unmarshal([]byte(record.ResponseSummary), out)
		out.Wave = wave
	}
	return out, nil
}

func (s *Service) CompleteFulfillmentWave(c *gin.Context, inv *inventory.Service, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, waveID uuid.UUID, in FulfillmentWaveRevisionInput) (*CompleteFulfillmentWaveResult, error) {
	if s == nil || s.DB == nil || s.Idempotency == nil || s.Idempotency.DB == nil || inv == nil || c == nil || tenantID < 0 || waveID == uuid.Nil {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	currentWave, err := s.GetFulfillmentWave(c.Request.Context(), tenantID, principal, waveID)
	if err != nil {
		return nil, err
	}
	if err := requireFulfillmentWaveOperate(principal, currentWave); err != nil {
		return nil, err
	}
	if _, err := validateWaveRevisionInput(in); err != nil {
		return nil, err
	}
	baseRevision := in.ExpectedRevision
	if currentWave.Status == FulfillmentWaveCompleting && currentWave.CompletionBaseRevision > 0 {
		baseRevision = currentWave.CompletionBaseRevision
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		TenantID int64     `json:"tenantId"`
		WaveID   uuid.UUID `json:"waveId"`
		Revision int       `json:"revision"`
	}{tenantID, waveID, baseRevision})
	if err != nil {
		return nil, err
	}
	// A completion run is identified by the wave revision that authorized it,
	// not by the browser-generated key. This permits a safe resume after a
	// refresh while preventing a second client key from starting the same run.
	idemKey := fmt.Sprintf("%d:%s:%d", tenantID, waveID, baseRevision)
	owner := fulfillmentWaveOwner("fulfillment-wave-complete", actor)
	acquired, acquireErr := s.Idempotency.Acquire(c.Request.Context(), fulfillmentWaveCompleteScope, idemKey, hash, owner, idempotency.DefaultLease)
	decision, record, classifyErr := idempotency.Classify(acquired, acquireErr)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return s.replayCompletedFulfillmentWave(c, tenantID, principal, waveID, record)
	case idempotency.DecisionInProgress:
		return nil, ErrFulfillmentWaveCompleting
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, ErrFulfillmentWaveIdempotency
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if acquired == nil || acquired.Record == nil {
			return nil, ErrFulfillmentWaveIdempotency
		}
	default:
		if classifyErr != nil {
			return nil, classifyErr
		}
		return nil, ErrFulfillmentWaveIdempotency
	}

	ctx := c.Request.Context()
	var candidates []FulfillmentWaveOrder
	var fulfillmentWarehouseID uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wave, err := getFulfillmentWaveTx(ctx, tx, tenantID, waveID)
		if err != nil {
			return err
		}
		if wave.Status == FulfillmentWaveCompleted {
			return ErrFulfillmentWaveState
		}
		fulfillmentWarehouseID = wave.WarehouseID
		if wave.Status == FulfillmentWaveCompleting {
			if wave.CompletionBaseRevision != baseRevision {
				return ErrFulfillmentWaveRevision
			}
			// A shared idempotency lease has authorized this explicit manual
			// resume. Per-order fulfillment keys make already-finished orders
			// replay-safe after an interrupted request.
		} else {
			if wave.Status != FulfillmentWavePacking && wave.Status != FulfillmentWavePartial {
				return ErrFulfillmentWaveState
			}
			if wave.Revision != in.ExpectedRevision {
				return ErrFulfillmentWaveRevision
			}
			var packed int64
			if err := tx.Model(&FulfillmentWaveOrder{}).Where("tenant_id = ? AND wave_id = ? AND status = ?", tenantID, waveID, FulfillmentWaveOrderPacked).Count(&packed).Error; err != nil {
				return err
			}
			if packed == 0 {
				return ErrFulfillmentWavePackingIncomplete
			}
			result := tx.Model(&FulfillmentWave{}).Where("id = ? AND tenant_id = ? AND revision = ?", waveID, tenantID, wave.Revision).Updates(map[string]any{
				"status": FulfillmentWaveCompleting, "revision": wave.Revision + 1,
				"completion_base_revision": wave.Revision, "updated_at": time.Now().UTC(),
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrFulfillmentWaveRevision
			}
		}
		return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND wave_id = ? AND status = ?", tenantID, waveID, FulfillmentWaveOrderPacked).Order("created_at ASC, id ASC").Find(&candidates).Error
	})
	if err != nil {
		_ = s.Idempotency.Fail(ctx, acquired.Record.ID, owner, clampWaveText(err.Error(), 64), true)
		return nil, err
	}

	out := &CompleteFulfillmentWaveResult{Items: make([]CompleteFulfillmentWaveItem, 0, len(candidates))}
	for _, candidate := range candidates {
		if heartbeatErr := s.Idempotency.Heartbeat(ctx, acquired.Record.ID, owner, idempotency.DefaultLease); heartbeatErr != nil {
			_ = s.Idempotency.Fail(ctx, acquired.Record.ID, owner, "lease_lost", true)
			return nil, heartbeatErr
		}
		item := CompleteFulfillmentWaveItem{OrderID: candidate.OrderID, OrderNo: candidate.OrderNo}
		result, fulfillErr := s.fulfillOrderForWave(c, inv, candidate.OrderID, FulfillOrderInput{
			IdempotencyKey: fmt.Sprintf("wave-%s-order-%s", waveID, candidate.OrderID),
			WarehouseID:    &fulfillmentWarehouseID,
			Carrier:        candidate.Carrier, TrackingNo: candidate.TrackingNo, TrackingURL: candidate.TrackingURL,
		}, actor, waveID)
		updateErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var locked FulfillmentWaveOrder
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND wave_id = ? AND id = ?", tenantID, waveID, candidate.ID).First(&locked).Error; err != nil {
				return err
			}
			if locked.Status == FulfillmentWaveOrderFulfilled {
				item.Status = FulfillmentWaveOrderFulfilled
				item.ShipmentID = locked.ShipmentID
				return nil
			}
			now := time.Now().UTC()
			if fulfillErr != nil {
				code := fulfillmentWaveFailureCode(fulfillErr)
				reason := clampWaveText(fulfillErr.Error(), 500)
				if err := tx.Model(&FulfillmentWaveOrder{}).Where("id = ? AND tenant_id = ?", locked.ID, tenantID).Updates(map[string]any{
					"status": FulfillmentWaveOrderFailed, "failure_code": code, "failure_reason": reason, "updated_at": now,
				}).Error; err != nil {
					return err
				}
				item.Status = FulfillmentWaveOrderFailed
				item.FailureReason = reason
				return nil
			}
			if result == nil || result.Shipment == nil || result.Shipment.ID == uuid.Nil {
				return fmt.Errorf("wave fulfillment succeeded without shipment")
			}
			shipmentID := result.Shipment.ID
			if err := tx.Model(&FulfillmentWaveOrder{}).Where("id = ? AND tenant_id = ?", locked.ID, tenantID).Updates(map[string]any{
				"status": FulfillmentWaveOrderFulfilled, "shipment_id": shipmentID, "fulfilled_at": now,
				"failure_code": "", "failure_reason": "", "updated_at": now,
			}).Error; err != nil {
				return err
			}
			if err := tx.Where("tenant_id = ? AND order_id = ? AND wave_id = ?", tenantID, candidate.OrderID, waveID).Delete(&FulfillmentWaveAssignment{}).Error; err != nil {
				return err
			}
			item.Status = FulfillmentWaveOrderFulfilled
			item.ShipmentID = &shipmentID
			return nil
		})
		if updateErr != nil {
			_ = s.Idempotency.Fail(ctx, acquired.Record.ID, owner, "wave_result_persist_failed", true)
			return nil, updateErr
		}
		out.Items = append(out.Items, item)
	}

	return s.finalizeFulfillmentWaveCompletion(c, tenantID, principal, actor, waveID, owner, acquired.Record.ID, out)
}

func (s *Service) finalizeFulfillmentWaveCompletion(c *gin.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, waveID uuid.UUID, owner string, recordID uuid.UUID, out *CompleteFulfillmentWaveResult) (*CompleteFulfillmentWaveResult, error) {
	ctx := c.Request.Context()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wave, err := getFulfillmentWaveTx(ctx, tx, tenantID, waveID)
		if err != nil {
			return err
		}
		var total, fulfilled, failed int64
		base := tx.Model(&FulfillmentWaveOrder{}).Where("tenant_id = ? AND wave_id = ?", tenantID, waveID)
		if err := base.Count(&total).Error; err != nil {
			return err
		}
		if err := tx.Model(&FulfillmentWaveOrder{}).Where("tenant_id = ? AND wave_id = ? AND status = ?", tenantID, waveID, FulfillmentWaveOrderFulfilled).Count(&fulfilled).Error; err != nil {
			return err
		}
		if err := tx.Model(&FulfillmentWaveOrder{}).Where("tenant_id = ? AND wave_id = ? AND status = ?", tenantID, waveID, FulfillmentWaveOrderFailed).Count(&failed).Error; err != nil {
			return err
		}
		status := FulfillmentWavePartial
		var completedAt *time.Time
		if total > 0 && fulfilled == total {
			status = FulfillmentWaveCompleted
			now := time.Now().UTC()
			completedAt = &now
		}
		updates := map[string]any{
			"status": status, "revision": wave.Revision + 1, "fulfilled_count": int(fulfilled), "failed_count": int(failed),
			"completion_base_revision": 0, "completed_by": nil, "completed_at": completedAt, "updated_at": time.Now().UTC(),
		}
		if status == FulfillmentWaveCompleted {
			updates["completed_by"] = actor
		}
		if err := tx.Model(&FulfillmentWave{}).Where("id = ? AND tenant_id = ?", waveID, tenantID).Updates(updates).Error; err != nil {
			return err
		}
		out.Processed = len(out.Items)
		out.Succeeded = 0
		out.Failed = 0
		for _, item := range out.Items {
			if item.Status == FulfillmentWaveOrderFulfilled {
				out.Succeeded++
			} else {
				out.Failed++
			}
		}
		summary, _ := json.Marshal(struct {
			Processed int                           `json:"processed"`
			Succeeded int                           `json:"succeeded"`
			Failed    int                           `json:"failed"`
			Items     []CompleteFulfillmentWaveItem `json:"items"`
		}{out.Processed, out.Succeeded, out.Failed, out.Items})
		return s.Idempotency.WithDB(tx).Complete(ctx, recordID, owner, idempotency.CompleteResult{
			ResponseCode: "FULFILLMENT_WAVE_COMPLETED", ResponseSummary: string(summary), ResourceType: "fulfillment_wave", ResourceID: waveID.String(),
		})
	})
	if err != nil {
		_ = s.Idempotency.Fail(ctx, recordID, owner, clampWaveText(err.Error(), 64), true)
		return nil, err
	}
	wave, err := s.GetFulfillmentWave(ctx, tenantID, principal, waveID)
	if err != nil {
		return nil, err
	}
	out.Wave = wave
	s.writeFulfillmentWaveLog(ctx, tenantID, actor, waveID, "complete", fmt.Sprintf("succeeded=%d failed=%d", out.Succeeded, out.Failed))
	return out, nil
}

func fulfillmentWaveFailureCode(err error) string {
	switch {
	case errors.Is(err, ErrFulfillmentNotPaid):
		return "order_not_paid"
	case errors.Is(err, ErrFulfillmentAlreadyCompleted):
		return "order_already_fulfilled"
	case errors.Is(err, inventory.ErrInsufficientSKUStock):
		return "inventory_insufficient"
	case errors.Is(err, inventory.ErrOrderInventoryState):
		return "inventory_state_conflict"
	case errors.Is(err, idempotency.ErrKeyConflict):
		return "idempotency_conflict"
	default:
		return "fulfillment_failed"
	}
}

package order

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func getFulfillmentWaveTx(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID) (*FulfillmentWave, error) {
	var row FulfillmentWave
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFulfillmentWaveNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func findFulfillmentWaveActionTx(tx *gorm.DB, tenantID int64, waveID uuid.UUID, key, hash string) (bool, error) {
	var done FulfillmentWaveAction
	err := tx.Where("tenant_id = ? AND wave_id = ? AND idempotency_key = ?", tenantID, waveID, key).First(&done).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if done.RequestHash != hash {
		return false, ErrFulfillmentWaveIdempotency
	}
	return true, nil
}

func createFulfillmentWaveActionTx(tx *gorm.DB, tenantID int64, waveID uuid.UUID, action, key, hash string, expectedRevision, resultRevision int, actor *uuid.UUID) (*FulfillmentWaveAction, error) {
	row := &FulfillmentWaveAction{
		TenantID: tenantID, WaveID: waveID, Action: action, IdempotencyKey: key, RequestHash: hash,
		ExpectedRevision: expectedRevision, ResultRevision: resultRevision, ActorID: actor,
	}
	err := tx.Create(row).Error
	if isFulfillmentWaveUniqueViolation(err) {
		return nil, ErrFulfillmentWaveIdempotency
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func validateWaveRevisionInput(in FulfillmentWaveRevisionInput) (string, error) {
	key, err := normalizeWaveKey(in.IdempotencyKey)
	if err != nil || in.ExpectedRevision < 1 {
		return "", ErrFulfillmentWaveInvalidInput
	}
	return key, nil
}

func (s *Service) StartFulfillmentWave(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, id uuid.UUID, in FulfillmentWaveRevisionInput) (*FulfillmentWave, error) {
	waveView, err := s.GetFulfillmentWave(ctx, tenantID, principal, id)
	if err != nil {
		return nil, err
	}
	if err := requireFulfillmentWaveOperate(principal, waveView); err != nil {
		return nil, err
	}
	key, err := validateWaveRevisionInput(in)
	if err != nil {
		return nil, err
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		Action   string `json:"action"`
		Revision int    `json:"revision"`
	}{"start", in.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wave, err := getFulfillmentWaveTx(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if replay, err := findFulfillmentWaveActionTx(tx, tenantID, id, key, hash); err != nil || replay {
			return err
		}
		if wave.Revision != in.ExpectedRevision {
			return ErrFulfillmentWaveRevision
		}
		if wave.Status != FulfillmentWaveDraft {
			return ErrFulfillmentWaveState
		}
		now := time.Now().UTC()
		resultRevision := wave.Revision + 1
		result := tx.Model(&FulfillmentWave{}).Where("id = ? AND tenant_id = ? AND revision = ?", id, tenantID, wave.Revision).Updates(map[string]any{
			"status": FulfillmentWavePicking, "revision": resultRevision, "started_by": actor, "started_at": now, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrFulfillmentWaveRevision
		}
		_, err = createFulfillmentWaveActionTx(tx, tenantID, id, "start", key, hash, in.ExpectedRevision, resultRevision, actor)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.writeFulfillmentWaveLog(ctx, tenantID, actor, id, "start", "")
	return s.GetFulfillmentWave(ctx, tenantID, principal, id)
}

func (s *Service) RecordFulfillmentWavePick(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, id uuid.UUID, in RecordFulfillmentWavePickInput) (*FulfillmentWave, error) {
	waveView, err := s.GetFulfillmentWave(ctx, tenantID, principal, id)
	if err != nil {
		return nil, err
	}
	if err := requireFulfillmentWaveOperate(principal, waveView); err != nil {
		return nil, err
	}
	key, err := normalizeWaveKey(in.IdempotencyKey)
	if err != nil || in.ExpectedRevision < 1 || len(in.Lines) == 0 || len(in.Lines) > 500 {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	lines := append([]FulfillmentWavePickLineInput(nil), in.Lines...)
	seen := make(map[uuid.UUID]struct{}, len(lines))
	for _, line := range lines {
		if line.LineID == uuid.Nil || line.PickedQty < 0 || line.ShortageQty < 0 {
			return nil, ErrFulfillmentWaveInvalidInput
		}
		if _, ok := seen[line.LineID]; ok {
			return nil, ErrFulfillmentWaveInvalidInput
		}
		seen[line.LineID] = struct{}{}
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		Action   string                         `json:"action"`
		Revision int                            `json:"revision"`
		Lines    []FulfillmentWavePickLineInput `json:"lines"`
	}{"record_pick", in.ExpectedRevision, lines})
	if err != nil {
		return nil, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wave, err := getFulfillmentWaveTx(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if replay, err := findFulfillmentWaveActionTx(tx, tenantID, id, key, hash); err != nil || replay {
			return err
		}
		if wave.Revision != in.ExpectedRevision {
			return ErrFulfillmentWaveRevision
		}
		if wave.Status != FulfillmentWavePicking && wave.Status != FulfillmentWavePacking && wave.Status != FulfillmentWavePartial {
			return ErrFulfillmentWaveState
		}
		lineIDs := make([]uuid.UUID, 0, len(lines))
		for _, line := range lines {
			lineIDs = append(lineIDs, line.LineID)
		}
		var rows []FulfillmentWaveLine
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND wave_id = ? AND id IN ?", tenantID, id, lineIDs).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(lines) {
			return ErrFulfillmentWaveInvalidInput
		}
		byID := make(map[uuid.UUID]FulfillmentWavePickLineInput, len(lines))
		for _, input := range lines {
			byID[input.LineID] = input
		}
		var affectedOrderIDs []uuid.UUID
		for _, row := range rows {
			input := byID[row.ID]
			if input.PickedQty+input.ShortageQty != row.RequiredQty {
				return ErrFulfillmentWavePickIncomplete
			}
			if scanned := strings.TrimSpace(input.ScannedBarcode); row.Barcode != "" && !strings.EqualFold(scanned, row.Barcode) {
				return ErrFulfillmentWaveScanMismatch
			}
			if scanned := strings.TrimSpace(input.ScannedLocationCode); row.LocationCode != "" && !strings.EqualFold(scanned, row.LocationCode) {
				return ErrFulfillmentWaveScanMismatch
			}
			var waveOrder FulfillmentWaveOrder
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND wave_id = ?", tenantID, row.WaveOrderID, id).First(&waveOrder).Error; err != nil {
				return err
			}
			if waveOrder.Status == FulfillmentWaveOrderPacked || waveOrder.Status == FulfillmentWaveOrderFulfilled {
				return ErrFulfillmentWaveState
			}
			status := FulfillmentWaveLinePicked
			if input.ShortageQty > 0 {
				status = FulfillmentWaveLineShortage
			}
			if err := tx.Model(&FulfillmentWaveLine{}).Where("id = ? AND tenant_id = ? AND wave_id = ?", row.ID, tenantID, id).Updates(map[string]any{
				"picked_qty": input.PickedQty, "shortage_qty": input.ShortageQty, "status": status, "updated_at": time.Now().UTC(),
			}).Error; err != nil {
				return err
			}
			affectedOrderIDs = append(affectedOrderIDs, row.OrderID)
		}
		for _, orderID := range uniqueWaveUUIDs(affectedOrderIDs) {
			if err := recomputeFulfillmentWaveOrderPickTx(tx, tenantID, id, orderID); err != nil {
				return err
			}
		}
		summary, err := fulfillmentWaveSummaryTx(tx, tenantID, id)
		if err != nil {
			return err
		}
		status := FulfillmentWavePicking
		if summary.pendingLines == 0 {
			status = FulfillmentWavePacking
		}
		if summary.fulfilledOrders > 0 || summary.failedOrders > 0 {
			status = FulfillmentWavePartial
		}
		resultRevision := wave.Revision + 1
		result := tx.Model(&FulfillmentWave{}).Where("id = ? AND tenant_id = ? AND revision = ?", id, tenantID, wave.Revision).Updates(map[string]any{
			"status": status, "revision": resultRevision, "picked_qty": summary.pickedQty, "shortage_qty": summary.shortageQty,
			"fulfilled_count": summary.fulfilledOrders, "failed_count": summary.failedOrders, "updated_at": time.Now().UTC(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrFulfillmentWaveRevision
		}
		action, err := createFulfillmentWaveActionTx(tx, tenantID, id, "record_pick", key, hash, in.ExpectedRevision, resultRevision, actor)
		if err != nil {
			return err
		}
		scans := make([]FulfillmentWavePickScan, 0, len(lines))
		for _, row := range rows {
			input := byID[row.ID]
			scans = append(scans, FulfillmentWavePickScan{
				TenantID: tenantID, WaveID: id, WaveLineID: row.ID, ActionID: action.ID,
				ExpectedBarcode: row.Barcode, ScannedBarcode: strings.TrimSpace(input.ScannedBarcode),
				ExpectedLocation: row.LocationCode, ScannedLocation: strings.TrimSpace(input.ScannedLocationCode),
				PickedQty: input.PickedQty, ShortageQty: input.ShortageQty, Validated: true, ActorID: actor,
			})
		}
		return tx.Create(&scans).Error
	})
	if err != nil {
		return nil, err
	}
	scannedCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line.ScannedBarcode) != "" || strings.TrimSpace(line.ScannedLocationCode) != "" {
			scannedCount++
		}
	}
	s.writeFulfillmentWaveLog(ctx, tenantID, actor, id, "record_pick", fmt.Sprintf("lineCount=%d scannedLineCount=%d", len(lines), scannedCount))
	return s.GetFulfillmentWave(ctx, tenantID, principal, id)
}

func uniqueWaveUUIDs(values []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(values))
	out := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func recomputeFulfillmentWaveOrderPickTx(tx *gorm.DB, tenantID int64, waveID, orderID uuid.UUID) error {
	var waveOrder FulfillmentWaveOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND wave_id = ? AND order_id = ?", tenantID, waveID, orderID).First(&waveOrder).Error; err != nil {
		return err
	}
	if waveOrder.Status == FulfillmentWaveOrderPacked || waveOrder.Status == FulfillmentWaveOrderFulfilled {
		return nil
	}
	var lines []FulfillmentWaveLine
	if err := tx.Where("tenant_id = ? AND wave_id = ? AND order_id = ?", tenantID, waveID, orderID).Find(&lines).Error; err != nil {
		return err
	}
	status := FulfillmentWaveOrderReadyToPack
	for _, line := range lines {
		if line.Status == FulfillmentWaveLinePending {
			status = FulfillmentWaveOrderPending
			break
		}
		if line.Status == FulfillmentWaveLineShortage {
			status = FulfillmentWaveOrderBlocked
		}
	}
	return tx.Model(&FulfillmentWaveOrder{}).Where("id = ? AND tenant_id = ?", waveOrder.ID, tenantID).Updates(map[string]any{
		"status": status, "failure_code": "", "failure_reason": "", "updated_at": time.Now().UTC(),
	}).Error
}

type waveSummary struct {
	pickedQty       int
	shortageQty     int
	pendingLines    int64
	fulfilledOrders int
	failedOrders    int
}

func fulfillmentWaveSummaryTx(tx *gorm.DB, tenantID int64, waveID uuid.UUID) (waveSummary, error) {
	var summary waveSummary
	var lines []FulfillmentWaveLine
	if err := tx.Where("tenant_id = ? AND wave_id = ?", tenantID, waveID).Find(&lines).Error; err != nil {
		return summary, err
	}
	for _, line := range lines {
		summary.pickedQty += line.PickedQty
		summary.shortageQty += line.ShortageQty
		if line.Status == FulfillmentWaveLinePending {
			summary.pendingLines++
		}
	}
	var orders []FulfillmentWaveOrder
	if err := tx.Where("tenant_id = ? AND wave_id = ?", tenantID, waveID).Find(&orders).Error; err != nil {
		return summary, err
	}
	for _, row := range orders {
		if row.Status == FulfillmentWaveOrderFulfilled {
			summary.fulfilledOrders++
		}
		if row.Status == FulfillmentWaveOrderFailed {
			summary.failedOrders++
		}
	}
	return summary, nil
}

func (s *Service) PackFulfillmentWaveOrder(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, waveID, orderID uuid.UUID, in PackFulfillmentWaveOrderInput) (*FulfillmentWave, error) {
	waveView, err := s.GetFulfillmentWave(ctx, tenantID, principal, waveID)
	if err != nil {
		return nil, err
	}
	if err := requireFulfillmentWaveOperate(principal, waveView); err != nil {
		return nil, err
	}
	key, err := normalizeWaveKey(in.IdempotencyKey)
	carrier := clampWaveText(in.Carrier, 128)
	trackingNo := clampWaveText(in.TrackingNo, 255)
	trackingURL := strings.TrimSpace(in.TrackingURL)
	if err != nil || in.ExpectedRevision < 1 || orderID == uuid.Nil || validateWaveTracking(carrier, trackingNo, trackingURL) != nil {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		Action      string    `json:"action"`
		Revision    int       `json:"revision"`
		OrderID     uuid.UUID `json:"orderId"`
		Carrier     string    `json:"carrier"`
		TrackingNo  string    `json:"trackingNo"`
		TrackingURL string    `json:"trackingUrl"`
	}{"pack", in.ExpectedRevision, orderID, carrier, trackingNo, trackingURL})
	if err != nil {
		return nil, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wave, err := getFulfillmentWaveTx(ctx, tx, tenantID, waveID)
		if err != nil {
			return err
		}
		if replay, err := findFulfillmentWaveActionTx(tx, tenantID, waveID, key, hash); err != nil || replay {
			return err
		}
		if wave.Revision != in.ExpectedRevision {
			return ErrFulfillmentWaveRevision
		}
		if wave.Status != FulfillmentWavePacking && wave.Status != FulfillmentWavePartial {
			return ErrFulfillmentWaveState
		}
		var waveOrder FulfillmentWaveOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND wave_id = ? AND order_id = ?", tenantID, waveID, orderID).First(&waveOrder).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFulfillmentWaveNotFound
			}
			return err
		}
		if waveOrder.Status != FulfillmentWaveOrderReadyToPack && waveOrder.Status != FulfillmentWaveOrderFailed && waveOrder.Status != FulfillmentWaveOrderPacked {
			return ErrFulfillmentWavePackingIncomplete
		}
		now := time.Now().UTC()
		if err := tx.Model(&FulfillmentWaveOrder{}).Where("id = ? AND tenant_id = ?", waveOrder.ID, tenantID).Updates(map[string]any{
			"status": FulfillmentWaveOrderPacked, "carrier": carrier, "tracking_no": trackingNo, "tracking_url": trackingURL,
			"failure_code": "", "failure_reason": "", "packed_by": actor, "packed_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		resultRevision := wave.Revision + 1
		result := tx.Model(&FulfillmentWave{}).Where("id = ? AND tenant_id = ? AND revision = ?", waveID, tenantID, wave.Revision).Updates(map[string]any{
			"revision": resultRevision, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrFulfillmentWaveRevision
		}
		_, err = createFulfillmentWaveActionTx(tx, tenantID, waveID, "pack", key, hash, in.ExpectedRevision, resultRevision, actor)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.writeFulfillmentWaveLog(ctx, tenantID, actor, waveID, "pack", "orderId="+orderID.String())
	return s.GetFulfillmentWave(ctx, tenantID, principal, waveID)
}

func (s *Service) CancelFulfillmentWave(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, id uuid.UUID, in FulfillmentWaveRevisionInput) (*FulfillmentWave, error) {
	waveView, err := s.GetFulfillmentWave(ctx, tenantID, principal, id)
	if err != nil {
		return nil, err
	}
	if err := requireFulfillmentWaveOperate(principal, waveView); err != nil {
		return nil, err
	}
	key, err := validateWaveRevisionInput(in)
	if err != nil {
		return nil, err
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		Action   string `json:"action"`
		Revision int    `json:"revision"`
	}{"cancel", in.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wave, err := getFulfillmentWaveTx(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if replay, err := findFulfillmentWaveActionTx(tx, tenantID, id, key, hash); err != nil || replay {
			return err
		}
		if wave.Revision != in.ExpectedRevision {
			return ErrFulfillmentWaveRevision
		}
		if wave.Status == FulfillmentWaveCompleted || wave.Status == FulfillmentWaveCancelled || wave.Status == FulfillmentWaveCompleting || wave.FulfilledCount > 0 {
			return ErrFulfillmentWaveState
		}
		var fulfilled int64
		if err := tx.Model(&FulfillmentWaveOrder{}).Where("tenant_id = ? AND wave_id = ? AND status = ?", tenantID, id, FulfillmentWaveOrderFulfilled).Count(&fulfilled).Error; err != nil {
			return err
		}
		if fulfilled > 0 {
			return ErrFulfillmentWaveState
		}
		now := time.Now().UTC()
		resultRevision := wave.Revision + 1
		result := tx.Model(&FulfillmentWave{}).Where("id = ? AND tenant_id = ? AND revision = ?", id, tenantID, wave.Revision).Updates(map[string]any{
			"status": FulfillmentWaveCancelled, "revision": resultRevision, "cancelled_by": actor, "cancelled_at": now, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrFulfillmentWaveRevision
		}
		if err := tx.Where("tenant_id = ? AND wave_id = ?", tenantID, id).Delete(&FulfillmentWaveAssignment{}).Error; err != nil {
			return err
		}
		_, err = createFulfillmentWaveActionTx(tx, tenantID, id, "cancel", key, hash, in.ExpectedRevision, resultRevision, actor)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.writeFulfillmentWaveLog(ctx, tenantID, actor, id, "cancel", "reservation retained")
	return s.GetFulfillmentWave(ctx, tenantID, principal, id)
}

func (s *Service) writeFulfillmentWaveLog(ctx context.Context, tenantID int64, actor *uuid.UUID, id uuid.UUID, action, message string) {
	if s == nil || s.OpLog == nil {
		return
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: actor, Action: "order.fulfillment_wave." + action,
		Resource: "fulfillment_wave", ResourceID: id.String(), Status: "success", Message: message,
	})
}

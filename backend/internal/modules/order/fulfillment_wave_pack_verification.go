package order

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxFulfillmentWavePackLines   = 500
	maxFulfillmentWaveWeightGrams = 5_000_000
)

func expectedFulfillmentWavePackCode(line FulfillmentWaveLine) string {
	if code := strings.TrimSpace(line.Barcode); code != "" {
		return code
	}
	return strings.TrimSpace(line.SKUCode)
}

// VerifyFulfillmentWavePack atomically validates the frozen order/SKU/package
// facts and records immutable audit rows before marking one order packed. It
// performs no carrier, marketplace, printer, scale, or inventory write.
func (s *Service) VerifyFulfillmentWavePack(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, waveID, orderID uuid.UUID, in VerifyFulfillmentWavePackInput) (*FulfillmentWave, error) {
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
	packageCode := clampWaveText(in.PackageCode, 255)
	scannedOrderNo := clampWaveText(in.ScannedOrderNo, 128)
	if err != nil || in.ExpectedRevision < 1 || orderID == uuid.Nil || scannedOrderNo == "" || packageCode == "" ||
		validateWaveTracking(carrier, trackingNo, trackingURL) != nil || len(in.Lines) == 0 || len(in.Lines) > maxFulfillmentWavePackLines {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	if !strings.EqualFold(packageCode, trackingNo) {
		return nil, ErrFulfillmentWavePackageMismatch
	}
	if in.ActualWeightGrams != nil && (*in.ActualWeightGrams <= 0 || *in.ActualWeightGrams > maxFulfillmentWaveWeightGrams) {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	lines := append([]FulfillmentWavePackLineInput(nil), in.Lines...)
	sort.Slice(lines, func(i, j int) bool { return lines[i].LineID.String() < lines[j].LineID.String() })
	for i := range lines {
		lines[i].ScannedCode = clampWaveText(lines[i].ScannedCode, 128)
		if lines[i].LineID == uuid.Nil || lines[i].ScannedCode == "" || lines[i].VerifiedQty <= 0 ||
			(i > 0 && lines[i].LineID == lines[i-1].LineID) {
			return nil, ErrFulfillmentWaveInvalidInput
		}
	}
	hash, err := fulfillmentWavePayloadHash(struct {
		Action            string                         `json:"action"`
		Revision          int                            `json:"revision"`
		OrderID           uuid.UUID                      `json:"orderId"`
		ScannedOrderNo    string                         `json:"scannedOrderNo"`
		Carrier           string                         `json:"carrier"`
		TrackingNo        string                         `json:"trackingNo"`
		TrackingURL       string                         `json:"trackingUrl"`
		PackageCode       string                         `json:"packageCode"`
		ActualWeightGrams *int                           `json:"actualWeightGrams,omitempty"`
		Lines             []FulfillmentWavePackLineInput `json:"lines"`
	}{"verify_pack", in.ExpectedRevision, orderID, scannedOrderNo, carrier, trackingNo, trackingURL, packageCode, in.ActualWeightGrams, lines})
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
		if !wave.PackingVerificationRequired {
			return ErrFulfillmentWaveState
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
		if !strings.EqualFold(strings.TrimSpace(waveOrder.OrderNo), scannedOrderNo) {
			return ErrFulfillmentWaveOrderScanMismatch
		}
		if waveOrder.Status != FulfillmentWaveOrderReadyToPack && waveOrder.Status != FulfillmentWaveOrderFailed {
			return ErrFulfillmentWavePackingIncomplete
		}
		var storedLines []FulfillmentWaveLine
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND wave_id = ? AND wave_order_id = ?", tenantID, waveID, waveOrder.ID).Order("id ASC").Find(&storedLines).Error; err != nil {
			return err
		}
		if len(storedLines) == 0 || len(storedLines) != len(lines) {
			return ErrFulfillmentWavePackingIncomplete
		}
		providedByID := make(map[uuid.UUID]FulfillmentWavePackLineInput, len(lines))
		for _, line := range lines {
			providedByID[line.LineID] = line
		}
		verifiedQty := 0
		for _, stored := range storedLines {
			provided, ok := providedByID[stored.ID]
			expectedCode := expectedFulfillmentWavePackCode(stored)
			if !ok || expectedCode == "" || !strings.EqualFold(expectedCode, provided.ScannedCode) {
				return ErrFulfillmentWavePackScanMismatch
			}
			if stored.Status != FulfillmentWaveLinePicked || provided.VerifiedQty != stored.RequiredQty {
				return ErrFulfillmentWavePackingIncomplete
			}
			verifiedQty += provided.VerifiedQty
		}

		now := time.Now().UTC()
		if err := tx.Model(&FulfillmentWaveOrder{}).Where("id = ? AND tenant_id = ?", waveOrder.ID, tenantID).Updates(map[string]any{
			"status": FulfillmentWaveOrderPacked, "carrier": carrier, "tracking_no": trackingNo, "tracking_url": trackingURL,
			"package_code": packageCode, "actual_weight_grams": in.ActualWeightGrams,
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
		action, err := createFulfillmentWaveActionTx(tx, tenantID, waveID, "verify_pack", key, hash, in.ExpectedRevision, resultRevision, actor)
		if err != nil {
			return err
		}
		verification := FulfillmentWavePackVerification{
			TenantID: tenantID, WaveID: waveID, WaveOrderID: waveOrder.ID, OrderID: orderID, ActionID: action.ID,
			PackageCode: packageCode, ScannedOrderNo: scannedOrderNo, Carrier: carrier, TrackingNo: trackingNo,
			TrackingURL: trackingURL, ActualWeightGrams: in.ActualWeightGrams, LineCount: len(storedLines), VerifiedQty: verifiedQty, ActorID: actor,
		}
		if err := tx.Create(&verification).Error; err != nil {
			return err
		}
		scans := make([]FulfillmentWavePackScan, 0, len(storedLines))
		for _, stored := range storedLines {
			provided := providedByID[stored.ID]
			scans = append(scans, FulfillmentWavePackScan{
				TenantID: tenantID, WaveID: waveID, WaveOrderID: waveOrder.ID, WaveLineID: stored.ID,
				VerificationID: verification.ID, ExpectedCode: expectedFulfillmentWavePackCode(stored), ScannedCode: provided.ScannedCode,
				ExpectedQty: stored.RequiredQty, VerifiedQty: provided.VerifiedQty, Validated: true, ActorID: actor,
			})
		}
		return tx.Create(&scans).Error
	})
	if err != nil {
		return nil, err
	}
	s.writeFulfillmentWaveLog(ctx, tenantID, actor, waveID, "verify_pack", fmt.Sprintf("orderId=%s lineCount=%d", orderID, len(lines)))
	return s.GetFulfillmentWave(ctx, tenantID, principal, waveID)
}

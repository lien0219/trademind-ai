package freightfee

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	ordermodule "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidInput = errors.New("invalid carrier freight input")
	ErrNotFound     = errors.New("carrier freight record not found")
	ErrConflict     = errors.New("carrier freight source, calculation, or idempotency conflict")
)

type Service struct {
	DB    *gorm.DB
	Clock func() time.Time
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func actorPointer(actor *uuid.UUID) *uuid.UUID {
	if actor == nil || *actor == uuid.Nil {
		return nil
	}
	value := *actor
	return &value
}

func hashValue(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func safeAdd(left, right int64) (int64, bool) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, false
	}
	value := left + right
	return value, value >= -MaxJSONSafeInt64 && value <= MaxJSONSafeInt64
}

func normalizeKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 8 || len(value) > 128 {
		return "", ErrInvalidInput
	}
	return value, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") || strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "unique constraint") || strings.Contains(message, "unique violation") ||
		strings.Contains(message, "constraint failed") || strings.Contains(message, "sqlstate 23505") ||
		strings.Contains(message, "error 1062")
}

func scopeAllowsShop(scope Scope, shopID uuid.UUID) bool {
	if !scope.RestrictStoreScope {
		return true
	}
	for _, allowed := range scope.AllowedShopIDs {
		if allowed == shopID {
			return true
		}
	}
	return false
}

func applyShopScope(query *gorm.DB, scope Scope, column string) *gorm.DB {
	if !scope.RestrictStoreScope {
		return query
	}
	if len(scope.AllowedShopIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where(column+" IN ?", scope.AllowedShopIDs)
}

func (s *Service) loadShop(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, lock bool) (*shop.Shop, error) {
	var row shop.Shop
	query := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, shopID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load freight fee shop: %w", err)
	}
	return &row, nil
}

type shipmentCandidate struct {
	ShipmentID uuid.UUID `gorm:"column:shipment_id"`
	OrderID    uuid.UUID `gorm:"column:order_id"`
	OrderNo    string    `gorm:"column:order_no"`
	Carrier    string    `gorm:"column:carrier"`
	Currency   string    `gorm:"column:currency"`
}

func findShipment(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, row CSVRow, lock bool) ([]shipmentCandidate, error) {
	var candidates []shipmentCandidate
	query := db.WithContext(ctx).Table("order_shipments AS shipments").
		Select("shipments.id AS shipment_id, orders.id AS order_id, orders.order_no, shipments.carrier, orders.currency").
		Joins("JOIN orders ON orders.id = shipments.order_id AND orders.tenant_id = ? AND orders.deleted_at IS NULL", tenantID).
		Where("orders.shop_id = ? AND shipments.tracking_no = ? AND shipments.status IN ?",
			shopID, row.TrackingNo, billableShipmentStatuses())
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Order("orders.id ASC, shipments.id ASC").Scan(&candidates).Error; err != nil {
		return nil, fmt.Errorf("match freight fee tracking number: %w", err)
	}
	matched := candidates[:0]
	carrier := normalizeCarrier(row.Carrier)
	for _, candidate := range candidates {
		if normalizeCarrier(candidate.Carrier) == carrier {
			matched = append(matched, candidate)
		}
	}
	return matched, nil
}

func (s *Service) Preview(ctx context.Context, tenantID int64, scope Scope, shopID uuid.UUID, fileName string, data []byte) (*PreviewResult, error) {
	if s == nil {
		return nil, ErrInvalidInput
	}
	return s.previewWithDB(ctx, s.DB, tenantID, scope, shopID, fileName, data, false)
}

type calculationFact struct {
	Line        int       `json:"line"`
	RowHash     string    `json:"rowHash"`
	ShipmentID  uuid.UUID `json:"shipmentId"`
	OrderID     uuid.UUID `json:"orderId"`
	Disposition string    `json:"disposition"`
}

func (s *Service) previewWithDB(ctx context.Context, db *gorm.DB, tenantID int64, scope Scope, shopID uuid.UUID, fileName string, data []byte, lock bool) (*PreviewResult, error) {
	if s == nil || db == nil || tenantID < 0 || shopID == uuid.Nil || len(data) == 0 || len(data) > MaxFileBytes || len(fileName) > 255 {
		return nil, ErrInvalidInput
	}
	if !scopeAllowsShop(scope, shopID) {
		return nil, ErrNotFound
	}
	shopRow, err := s.loadShop(ctx, db, tenantID, shopID, lock)
	if err != nil {
		return nil, err
	}
	rows, issues := parseCSV(data)
	result := &PreviewResult{
		FileName: strings.TrimSpace(fileName), FileHash: fileHash(data), PolicyVersion: PolicyVersion,
		ShopID: shopID, ShopName: shopRow.ShopName, SourceRows: sourceRowCount(rows, issues),
		Rows: make([]ChargePreview, 0, len(rows)), Issues: issues, Warnings: []ValidationIssue{},
	}
	calculationRows := make([]calculationFact, 0, len(rows))
	for _, row := range rows {
		previewRow := ChargePreview{CSVRow: row, Status: "blocked"}
		candidates, matchErr := findShipment(ctx, db, tenantID, shopID, row, lock)
		if matchErr != nil {
			return nil, matchErr
		}
		if len(candidates) == 0 {
			result.Issues = append(result.Issues, ValidationIssue{Line: row.Line, Field: CSVHeaders[2], Code: "shipment_not_found", Message: "未找到该店铺下承运商和运单号均一致的已履约包裹"})
			result.Rows = append(result.Rows, previewRow)
			continue
		}
		if len(candidates) != 1 {
			result.Issues = append(result.Issues, ValidationIssue{Line: row.Line, Field: CSVHeaders[2], Code: "shipment_ambiguous", Message: "该承运商和运单号匹配多个包裹，账单未确认"})
			result.Rows = append(result.Rows, previewRow)
			continue
		}
		candidate := candidates[0]
		previewRow.ShipmentID, previewRow.OrderID = &candidate.ShipmentID, &candidate.OrderID
		previewRow.OrderNo, previewRow.OrderCurrency = candidate.OrderNo, strings.ToUpper(strings.TrimSpace(candidate.Currency))
		previewRow.Status = "confirmed"
		var external Charge
		externalErr := db.WithContext(ctx).Where("tenant_id = ? AND shop_id = ? AND carrier = ? AND external_line_id = ?", tenantID, shopID, normalizeCarrier(row.Carrier), row.ExternalLineID).Take(&external).Error
		if externalErr != nil && !errors.Is(externalErr, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("lookup carrier freight line: %w", externalErr)
		}
		var shipmentCharge Charge
		shipmentErr := db.WithContext(ctx).Where("tenant_id = ? AND shipment_id = ?", tenantID, candidate.ShipmentID).Take(&shipmentCharge).Error
		if shipmentErr != nil && !errors.Is(shipmentErr, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("lookup billed freight shipment: %w", shipmentErr)
		}
		switch {
		case externalErr == nil && external.RowHash == row.RowHash && external.ShipmentID == candidate.ShipmentID:
			previewRow.Disposition = DispositionDup
			result.DuplicateCharges++
		case externalErr == nil:
			previewRow.Status = "blocked"
			result.Issues = append(result.Issues, ValidationIssue{Line: row.Line, Field: CSVHeaders[0], Code: "external_line_conflict", Message: "承运商费用编号已关联不同的账单事实"})
		case shipmentErr == nil:
			previewRow.Status = "blocked"
			result.Issues = append(result.Issues, ValidationIssue{Line: row.Line, Field: CSVHeaders[2], Code: "shipment_already_billed", Message: "该包裹已有最终运费实账；更正请使用追加调整"})
		default:
			previewRow.Disposition = DispositionNew
			result.NewCharges++
		}
		if previewRow.Status != "blocked" && previewRow.OrderCurrency != row.Currency {
			previewRow.Status, previewRow.ReasonCode, previewRow.Reason = "mismatch", "freight_currency_mismatch", "账单币种与订单币种不同；该实账不会替代利润中的本地运费估价"
			result.Warnings = append(result.Warnings, ValidationIssue{Line: row.Line, Field: CSVHeaders[4], Code: previewRow.ReasonCode, Message: previewRow.Reason})
		}
		calculationRows = append(calculationRows, calculationFact{Line: row.Line, RowHash: row.RowHash, ShipmentID: candidate.ShipmentID, OrderID: candidate.OrderID, Disposition: previewRow.Disposition})
		result.Rows = append(result.Rows, previewRow)
	}
	result.CalculationHash, err = hashValue(struct {
		PolicyVersion string            `json:"policyVersion"`
		ShopID        uuid.UUID         `json:"shopId"`
		FileHash      string            `json:"fileHash"`
		Rows          []calculationFact `json:"rows"`
	}{PolicyVersion, shopID, result.FileHash, calculationRows})
	if err != nil {
		return nil, fmt.Errorf("hash freight fee calculation: %w", err)
	}
	var prior Import
	priorErr := db.WithContext(ctx).Where("tenant_id = ? AND shop_id = ? AND file_hash = ?", tenantID, shopID, result.FileHash).Take(&prior).Error
	if priorErr != nil && !errors.Is(priorErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("lookup previous freight fee import: %w", priorErr)
	}
	if priorErr == nil {
		result.CalculationHash = prior.CalculationHash
	}
	result.Valid = len(result.Rows) > 0 && len(result.Issues) == 0
	return result, nil
}

func sourceRowCount(rows []CSVRow, issues []ValidationIssue) int {
	seen := make(map[int]struct{}, len(rows)+len(issues))
	for _, row := range rows {
		seen[row.Line] = struct{}{}
	}
	for _, issue := range issues {
		if issue.Line > 1 {
			seen[issue.Line] = struct{}{}
		}
	}
	return len(seen)
}

func fileHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA256Hash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func (s *Service) Confirm(ctx context.Context, tenantID int64, scope Scope, shopID uuid.UUID, fileName string, data []byte, expectedFileHash, expectedCalculationHash, idempotencyKey string, actor *uuid.UUID) (*ImportResult, error) {
	key, err := normalizeKey(idempotencyKey)
	if err != nil || len(expectedFileHash) != 64 || len(expectedCalculationHash) != 64 {
		return nil, ErrInvalidInput
	}
	expectedFileHash = strings.ToLower(expectedFileHash)
	expectedCalculationHash = strings.ToLower(expectedCalculationHash)
	if fileHash(data) != expectedFileHash {
		return nil, ErrConflict
	}
	requestHash, err := hashValue(struct {
		TenantID int64     `json:"tenantId"`
		ShopID   uuid.UUID `json:"shopId"`
		FileHash string    `json:"fileHash"`
		CalcHash string    `json:"calculationHash"`
	}{tenantID, shopID, expectedFileHash, expectedCalculationHash})
	if err != nil {
		return nil, err
	}
	if !scopeAllowsShop(scope, shopID) {
		return nil, ErrNotFound
	}
	var result *ImportResult
	if s == nil || s.DB == nil {
		return nil, ErrInvalidInput
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, lockErr := s.loadShop(ctx, tx, tenantID, shopID, true); lockErr != nil {
			return lockErr
		}
		var existing Import
		if findErr := tx.WithContext(ctx).Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).Take(&existing).Error; findErr == nil {
			if existing.RequestHash != requestHash || existing.ShopID != shopID {
				return ErrConflict
			}
			result = &ImportResult{Import: existing, Replayed: true}
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if !scopeAllowsShop(scope, shopID) {
			return ErrNotFound
		}
		var prior Import
		if findErr := tx.WithContext(ctx).Where("tenant_id = ? AND shop_id = ? AND file_hash = ?", tenantID, shopID, expectedFileHash).Take(&prior).Error; findErr == nil {
			if prior.CalculationHash != expectedCalculationHash {
				return ErrConflict
			}
			result = &ImportResult{Import: prior, Replayed: true}
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		preview, previewErr := s.previewWithDB(ctx, tx, tenantID, scope, shopID, fileName, data, true)
		if previewErr != nil {
			return previewErr
		}
		if !preview.Valid || preview.FileHash != expectedFileHash || preview.CalculationHash != expectedCalculationHash {
			return ErrConflict
		}
		now := s.now()
		batch := Import{
			TenantID: tenantID, ShopID: shopID, FileName: strings.TrimSpace(fileName), FileHash: expectedFileHash,
			PolicyVersion: PolicyVersion, CalculationHash: expectedCalculationHash, IdempotencyKey: key,
			RequestHash: requestHash, SourceRowCount: preview.SourceRows, ImportedRows: preview.NewCharges,
			DuplicateRows: preview.DuplicateCharges, ImportedBy: actorPointer(actor), ConfirmedAt: now,
		}
		if err := tx.Create(&batch).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrConflict
			}
			return fmt.Errorf("create freight fee import: %w", err)
		}
		for _, row := range preview.Rows {
			if row.Disposition != DispositionNew || row.OrderID == nil || row.ShipmentID == nil {
				continue
			}
			charge := Charge{
				TenantID: tenantID, ImportID: batch.ID, ShopID: shopID, OrderID: *row.OrderID, ShipmentID: *row.ShipmentID,
				OrderNo: row.OrderNo, OrderCurrency: row.OrderCurrency, Carrier: normalizeCarrier(row.Carrier), TrackingNo: row.TrackingNo,
				ExternalLineID: row.ExternalLineID, BilledAt: row.BilledAt.UTC(), AmountMinor: row.AmountMinor, Currency: row.Currency,
				RowHash: row.RowHash, ImportedBy: actorPointer(actor), ImportedAt: now,
			}
			if err := tx.Create(&charge).Error; err != nil {
				if isUniqueViolation(err) {
					return ErrConflict
				}
				return fmt.Errorf("create freight fee charge: %w", err)
			}
		}
		result = &ImportResult{Import: batch}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeListQuery(input ListQuery) (ListQuery, error) {
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.TrackingNo = strings.TrimSpace(input.TrackingNo)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 {
		input.PageSize = 20
	}
	if input.PageSize > MaxPageSize || len(input.OrderNo) > 128 || len(input.TrackingNo) > 255 || (input.Currency != "" && !validCurrency(input.Currency)) ||
		(input.Status != "" && input.Status != "confirmed" && input.Status != "mismatch") {
		return input, ErrInvalidInput
	}
	if input.Page > int(^uint(0)>>1)/input.PageSize {
		return input, ErrInvalidInput
	}
	return input, nil
}

func (s *Service) ListCharges(ctx context.Context, tenantID int64, scope Scope, raw ListQuery) (*ChargeList, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalidInput
	}
	queryInput, err := normalizeListQuery(raw)
	if err != nil {
		return nil, err
	}
	query := s.DB.WithContext(ctx).Model(&Charge{}).Where("tenant_id = ?", tenantID)
	query = applyShopScope(query, scope, "shop_id")
	if queryInput.OrderNo != "" {
		query = query.Where("order_no LIKE ?", "%"+escapeLike(queryInput.OrderNo)+"%")
	}
	if queryInput.TrackingNo != "" {
		query = query.Where("tracking_no = ?", queryInput.TrackingNo)
	}
	if queryInput.ShopID != nil {
		query = query.Where("shop_id = ?", *queryInput.ShopID)
	}
	if queryInput.Currency != "" {
		query = query.Where("currency = ?", queryInput.Currency)
	}
	if queryInput.Status == "confirmed" {
		query = query.Where("currency = order_currency")
	} else if queryInput.Status == "mismatch" {
		query = query.Where("currency <> order_currency")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count freight fee charges: %w", err)
	}
	var charges []Charge
	if err := query.Order("billed_at DESC, id DESC").Offset((queryInput.Page - 1) * queryInput.PageSize).Limit(queryInput.PageSize).Find(&charges).Error; err != nil {
		return nil, fmt.Errorf("list freight fee charges: %w", err)
	}
	adjustments, err := s.adjustmentsByCharge(ctx, tenantID, chargeIDs(charges))
	if err != nil {
		return nil, err
	}
	list := make([]ChargeSummary, 0, len(charges))
	for _, charge := range charges {
		summary, summaryErr := summarizeCharge(charge, adjustments[charge.ID])
		if summaryErr != nil {
			return nil, summaryErr
		}
		list = append(list, summary)
	}
	return &ChargeList{List: list, Page: queryInput.Page, PageSize: queryInput.PageSize, Total: total, TotalPages: pagesOf(total, queryInput.PageSize)}, nil
}

func (s *Service) GetCharge(ctx context.Context, tenantID int64, scope Scope, id uuid.UUID) (*ChargeDetail, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var charge Charge
	query := applyShopScope(s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id), scope, "shop_id")
	if err := query.Take(&charge).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load freight fee charge: %w", err)
	}
	adjustments, err := s.adjustmentsByCharge(ctx, tenantID, []uuid.UUID{charge.ID})
	if err != nil {
		return nil, err
	}
	rows := adjustments[charge.ID]
	summary, err := summarizeCharge(charge, rows)
	if err != nil {
		return nil, err
	}
	return &ChargeDetail{ChargeSummary: summary, Adjustments: rows}, nil
}

func (s *Service) ListImports(ctx context.Context, tenantID int64, scope Scope, page, pageSize int) (*ImportList, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalidInput
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > MaxPageSize {
		return nil, ErrInvalidInput
	}
	if page > int(^uint(0)>>1)/pageSize {
		return nil, ErrInvalidInput
	}
	query := s.DB.WithContext(ctx).Model(&Import{}).Where("tenant_id = ?", tenantID)
	query = applyShopScope(query, scope, "shop_id")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count freight fee imports: %w", err)
	}
	var rows []Import
	if err := query.Order("confirmed_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list freight fee imports: %w", err)
	}
	return &ImportList{List: rows, Page: page, PageSize: pageSize, Total: total, TotalPages: pagesOf(total, pageSize)}, nil
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

func chargeIDs(rows []Charge) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func (s *Service) adjustmentsByCharge(ctx context.Context, tenantID int64, ids []uuid.UUID) (map[uuid.UUID][]Adjustment, error) {
	result := make(map[uuid.UUID][]Adjustment, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []Adjustment
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND charge_id IN ?", tenantID, ids).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load freight fee adjustments: %w", err)
	}
	for _, row := range rows {
		result[row.ChargeID] = append(result[row.ChargeID], row)
	}
	return result, nil
}

func summarizeCharge(charge Charge, adjustments []Adjustment) (ChargeSummary, error) {
	result := ChargeSummary{Charge: charge, NetAmountMinor: charge.AmountMinor, Status: "confirmed"}
	if charge.ID == uuid.Nil || charge.OrderID == uuid.Nil || charge.ShipmentID == uuid.Nil || charge.AmountMinor < 0 || charge.AmountMinor > MaxJSONSafeInt64 ||
		!validCurrency(charge.Currency) || !validCurrency(charge.OrderCurrency) || !validSHA256Hash(charge.RowHash) || charge.ImportID == uuid.Nil ||
		charge.ShopID == uuid.Nil || strings.TrimSpace(charge.OrderNo) == "" || strings.TrimSpace(charge.Carrier) == "" || strings.TrimSpace(charge.TrackingNo) == "" {
		return result, ErrConflict
	}
	byID := make(map[uuid.UUID]Adjustment, len(adjustments))
	for _, row := range adjustments {
		byID[row.ID] = row
	}
	for _, row := range adjustments {
		if row.ID == uuid.Nil || row.ChargeID != charge.ID || row.OrderID != charge.OrderID || row.ShopID != charge.ShopID || row.AmountMinor == 0 ||
			row.AmountMinor < -MaxJSONSafeInt64 || row.AmountMinor > MaxJSONSafeInt64 || row.Currency != charge.Currency {
			return result, ErrConflict
		}
		switch row.FactType {
		case FactAdjustment:
			if row.ReversesAdjustmentID != nil {
				return result, ErrConflict
			}
		case FactReversal:
			if row.ReversesAdjustmentID == nil {
				return result, ErrConflict
			}
			target, ok := byID[*row.ReversesAdjustmentID]
			if !ok || target.FactType != FactAdjustment || target.ReversesAdjustmentID != nil || target.AmountMinor == math.MinInt64 || row.AmountMinor != -target.AmountMinor {
				return result, ErrConflict
			}
		default:
			return result, ErrConflict
		}
		next, ok := safeAdd(result.AdjustmentMinor, row.AmountMinor)
		if !ok {
			return result, ErrConflict
		}
		result.AdjustmentMinor = next
		result.AdjustmentCount++
	}
	net, ok := safeAdd(charge.AmountMinor, result.AdjustmentMinor)
	if !ok || net < 0 {
		return result, ErrConflict
	}
	result.NetAmountMinor = net
	if charge.Currency != charge.OrderCurrency {
		result.Status, result.ReasonCode, result.Reason = "mismatch", "freight_currency_mismatch", "账单币种与订单币种不一致"
	}
	return result, nil
}

func adjustmentRequestHash(chargeID uuid.UUID, factType string, amount int64, reason, key string, reverses *uuid.UUID) (string, error) {
	return hashValue(struct {
		ChargeID uuid.UUID  `json:"chargeId"`
		FactType string     `json:"factType"`
		Amount   int64      `json:"amountMinor"`
		Reason   string     `json:"reason"`
		Key      string     `json:"idempotencyKey"`
		Reverses *uuid.UUID `json:"reversesAdjustmentId,omitempty"`
	}{chargeID, factType, amount, reason, key, reverses})
}

func (s *Service) adjustmentReplay(ctx context.Context, tx *gorm.DB, tenantID int64, scope Scope, key, requestHash string) (*AdjustmentResult, error) {
	var existing Adjustment
	query := tx.WithContext(ctx).Table("freight_fee_adjustments AS adjustment").
		Select("adjustment.*").
		Joins("JOIN freight_fee_charges AS charge ON charge.id = adjustment.charge_id AND charge.tenant_id = adjustment.tenant_id").
		Where("adjustment.tenant_id = ? AND adjustment.idempotency_key = ?", tenantID, key)
	query = applyShopScope(query, scope, "charge.shop_id")
	if err := query.Take(&existing).Error; err != nil {
		return nil, err
	}
	if existing.RequestHash != requestHash {
		return nil, ErrConflict
	}
	return &AdjustmentResult{Adjustment: existing, Replayed: true}, nil
}

func (s *Service) lockedCharge(ctx context.Context, tx *gorm.DB, tenantID int64, scope Scope, id uuid.UUID) (*Charge, error) {
	var row Charge
	query := applyShopScope(tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id), scope, "shop_id")
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) validateNetAmount(ctx context.Context, tx *gorm.DB, charge Charge, delta int64) error {
	rowsByCharge, err := s.adjustmentsByChargeTx(ctx, tx, charge.TenantID, charge.ID)
	if err != nil {
		return err
	}
	current, err := summarizeCharge(charge, rowsByCharge)
	if err != nil {
		return ErrConflict
	}
	net, ok := safeAdd(current.NetAmountMinor, delta)
	if !ok || net < 0 {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) adjustmentsByChargeTx(ctx context.Context, tx *gorm.DB, tenantID int64, chargeID uuid.UUID) ([]Adjustment, error) {
	var rows []Adjustment
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND charge_id = ?", tenantID, chargeID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load freight fee adjustments for update: %w", err)
	}
	return rows, nil
}

func (s *Service) CreateAdjustment(ctx context.Context, tenantID int64, scope Scope, actor *uuid.UUID, chargeID uuid.UUID, input CreateAdjustmentInput) (*AdjustmentResult, error) {
	reason, key := strings.TrimSpace(input.Reason), strings.TrimSpace(input.IdempotencyKey)
	key, err := normalizeKey(key)
	if s == nil || s.DB == nil || tenantID < 0 || chargeID == uuid.Nil || input.AmountMinor == 0 || input.AmountMinor < -MaxJSONSafeInt64 || input.AmountMinor > MaxJSONSafeInt64 || len([]rune(reason)) < 2 || len([]rune(reason)) > 500 || err != nil {
		return nil, ErrInvalidInput
	}
	requestHash, err := adjustmentRequestHash(chargeID, FactAdjustment, input.AmountMinor, reason, key, nil)
	if err != nil {
		return nil, err
	}
	var result *AdjustmentResult
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if replay, findErr := s.adjustmentReplay(ctx, tx, tenantID, scope, key, requestHash); findErr == nil {
			result = replay
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		charge, findErr := s.lockedCharge(ctx, tx, tenantID, scope, chargeID)
		if findErr != nil {
			return findErr
		}
		if replay, findErr := s.adjustmentReplay(ctx, tx, tenantID, scope, key, requestHash); findErr == nil {
			result = replay
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if err := s.validateNetAmount(ctx, tx, *charge, input.AmountMinor); err != nil {
			return err
		}
		row := Adjustment{TenantID: tenantID, ChargeID: charge.ID, ShopID: charge.ShopID, OrderID: charge.OrderID, FactType: FactAdjustment,
			AmountMinor: input.AmountMinor, Currency: charge.Currency, Reason: reason, IdempotencyKey: key, RequestHash: requestHash, ActorID: actorPointer(actor)}
		if err := tx.Create(&row).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrConflict
			}
			return fmt.Errorf("create freight fee adjustment: %w", err)
		}
		result = &AdjustmentResult{Adjustment: row}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) ReverseAdjustment(ctx context.Context, tenantID int64, scope Scope, actor *uuid.UUID, chargeID, adjustmentID uuid.UUID, input ReverseAdjustmentInput) (*AdjustmentResult, error) {
	reason, key := strings.TrimSpace(input.Reason), strings.TrimSpace(input.IdempotencyKey)
	key, err := normalizeKey(key)
	if s == nil || s.DB == nil || tenantID < 0 || chargeID == uuid.Nil || adjustmentID == uuid.Nil || len([]rune(reason)) < 2 || len([]rune(reason)) > 500 || err != nil {
		return nil, ErrInvalidInput
	}
	requestHash, err := adjustmentRequestHash(chargeID, FactReversal, 0, reason, key, &adjustmentID)
	if err != nil {
		return nil, err
	}
	var result *AdjustmentResult
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if replay, findErr := s.adjustmentReplay(ctx, tx, tenantID, scope, key, requestHash); findErr == nil {
			result = replay
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		charge, findErr := s.lockedCharge(ctx, tx, tenantID, scope, chargeID)
		if findErr != nil {
			return findErr
		}
		if replay, findErr := s.adjustmentReplay(ctx, tx, tenantID, scope, key, requestHash); findErr == nil {
			result = replay
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		var target Adjustment
		if err := tx.WithContext(ctx).Where("tenant_id = ? AND charge_id = ? AND id = ? AND fact_type = ?", tenantID, chargeID, adjustmentID, FactAdjustment).Take(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if target.AmountMinor == math.MinInt64 {
			return ErrConflict
		}
		rows, err := s.adjustmentsByChargeTx(ctx, tx, tenantID, charge.ID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.ReversesAdjustmentID != nil && *row.ReversesAdjustmentID == target.ID {
				return ErrConflict
			}
		}
		delta := -target.AmountMinor
		if err := s.validateNetAmount(ctx, tx, *charge, delta); err != nil {
			return err
		}
		reversedID := target.ID
		row := Adjustment{TenantID: tenantID, ChargeID: charge.ID, ShopID: charge.ShopID, OrderID: charge.OrderID, FactType: FactReversal,
			AmountMinor: delta, Currency: charge.Currency, Reason: reason, ReversesAdjustmentID: &reversedID,
			IdempotencyKey: key, RequestHash: requestHash, ActorID: actorPointer(actor)}
		if err := tx.Create(&row).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrConflict
			}
			return fmt.Errorf("reverse freight fee adjustment: %w", err)
		}
		result = &AdjustmentResult{Adjustment: row}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type shipmentReference struct {
	ShipmentID uuid.UUID  `gorm:"column:shipment_id"`
	OrderID    uuid.UUID  `gorm:"column:order_id"`
	ShopID     *uuid.UUID `gorm:"column:shop_id"`
	OrderNo    string     `gorm:"column:order_no"`
	Carrier    string     `gorm:"column:carrier"`
	TrackingNo string     `gorm:"column:tracking_no"`
	Currency   string     `gorm:"column:currency"`
}

func billableShipmentStatuses() []string {
	return []string{
		ordermodule.ShipmentShipped,
		ordermodule.ShipmentInTransit,
		ordermodule.ShipmentDelivered,
		ordermodule.ShipmentException,
		ordermodule.ShipmentReturned,
	}
}

func chargeMatchesShipment(tenantID int64, charge Charge, shipment shipmentReference, batch Import) bool {
	return charge.TenantID == tenantID && charge.ID != uuid.Nil && charge.OrderID != uuid.Nil && charge.ShipmentID == shipment.ShipmentID &&
		charge.ImportID != uuid.Nil && charge.ShopID != uuid.Nil && shipment.OrderID == charge.OrderID && shipment.ShopID != nil && *shipment.ShopID == charge.ShopID &&
		strings.TrimSpace(charge.OrderNo) != "" && strings.TrimSpace(charge.Carrier) != "" && strings.TrimSpace(charge.TrackingNo) != "" &&
		strings.TrimSpace(shipment.OrderNo) == strings.TrimSpace(charge.OrderNo) &&
		normalizeCarrier(shipment.Carrier) == normalizeCarrier(charge.Carrier) &&
		strings.TrimSpace(shipment.TrackingNo) == strings.TrimSpace(charge.TrackingNo) &&
		strings.EqualFold(strings.TrimSpace(shipment.Currency), strings.TrimSpace(charge.OrderCurrency)) &&
		batch.ID == charge.ImportID && batch.TenantID == tenantID && batch.ShopID == charge.ShopID && batch.PolicyVersion == PolicyVersion &&
		validSHA256Hash(batch.FileHash) && validSHA256Hash(batch.CalculationHash)
}

func (s *Service) ProfitabilityFeesForOrders(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID]ProfitabilityFact, error) {
	result := make(map[uuid.UUID]ProfitabilityFact, len(orderIDs))
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalidInput
	}
	if len(orderIDs) == 0 {
		return result, nil
	}
	var shipmentRefs []shipmentReference
	if err := s.DB.WithContext(ctx).Table("order_shipments AS shipments").
		Select("shipments.id AS shipment_id, shipments.order_id, orders.shop_id, orders.order_no, shipments.carrier, shipments.tracking_no, orders.currency").
		Joins("JOIN orders ON orders.id = shipments.order_id AND orders.tenant_id = ? AND orders.deleted_at IS NULL", tenantID).
		Where("orders.id IN ? AND shipments.status IN ?", orderIDs, billableShipmentStatuses()).
		Order("orders.id ASC, shipments.id ASC").Scan(&shipmentRefs).Error; err != nil {
		return nil, fmt.Errorf("list freight fee shipments: %w", err)
	}
	shipmentByID := make(map[uuid.UUID]shipmentReference, len(shipmentRefs))
	for _, row := range shipmentRefs {
		shipmentByID[row.ShipmentID] = row
		fact := result[row.OrderID]
		fact.OrderID, fact.ShipmentCount = row.OrderID, fact.ShipmentCount+1
		result[row.OrderID] = fact
	}
	var charges []Charge
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND order_id IN ?", tenantID, orderIDs).Order("order_id ASC, shipment_id ASC, id ASC").Find(&charges).Error; err != nil {
		return nil, fmt.Errorf("list freight fees for profitability: %w", err)
	}
	importIDs := make([]uuid.UUID, 0, len(charges))
	for _, charge := range charges {
		importIDs = appendUniqueID(importIDs, charge.ImportID)
	}
	imports := make(map[uuid.UUID]Import, len(importIDs))
	if len(importIDs) > 0 {
		var batches []Import
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, importIDs).Find(&batches).Error; err != nil {
			return nil, fmt.Errorf("list freight fee import facts for profitability: %w", err)
		}
		for _, batch := range batches {
			imports[batch.ID] = batch
		}
	}
	adjustments, err := s.adjustmentsByCharge(ctx, tenantID, chargeIDs(charges))
	if err != nil {
		return nil, err
	}
	for _, charge := range charges {
		summary, summaryErr := summarizeCharge(charge, adjustments[charge.ID])
		fact := result[charge.OrderID]
		fact.OrderID = charge.OrderID
		fact.BilledShipmentCount++
		fact.ChargeIDs = append(fact.ChargeIDs, charge.ID)
		fact.ImportIDs = appendUniqueID(fact.ImportIDs, charge.ImportID)
		shipmentRef, shipmentExists := shipmentByID[charge.ShipmentID]
		batch, importExists := imports[charge.ImportID]
		if !shipmentExists || !importExists || !chargeMatchesShipment(tenantID, charge, shipmentRef, batch) {
			fact.Status, fact.ReasonCode, fact.Reason = "blocked", "freight_fact_scope_invalid", "运费实账的租户、店铺、订单、包裹或导入来源关联无效"
		}
		for _, adjustment := range adjustments[charge.ID] {
			fact.AdjustmentIDs = append(fact.AdjustmentIDs, adjustment.ID)
			if adjustment.CreatedAt.After(fact.SourceAt) {
				fact.SourceAt = adjustment.CreatedAt.UTC()
			}
		}
		if charge.BilledAt.After(fact.SourceAt) {
			fact.SourceAt = charge.BilledAt.UTC()
		}
		if summaryErr != nil {
			fact.Status, fact.ReasonCode, fact.Reason = "blocked", "freight_fact_invalid", "运费实账或更正事实结构无效"
			result[charge.OrderID] = fact
			continue
		}
		if fact.Status != "blocked" {
			if len(fact.ChargeIDs) == 1 {
				fact.Currency = charge.Currency
			} else if fact.Currency != charge.Currency {
				fact.Status, fact.ReasonCode, fact.Reason = "mismatch", "freight_currency_mismatch", "同一订单的包裹运费币种不一致"
			}
			if summary.Status != "confirmed" {
				fact.Status, fact.ReasonCode, fact.Reason = summary.Status, summary.ReasonCode, summary.Reason
			}
		}
		if fact.Status != "blocked" && fact.Status != "mismatch" {
			if next, ok := safeAdd(fact.AmountMinor, summary.NetAmountMinor); ok {
				fact.AmountMinor = next
			} else {
				fact.Status, fact.ReasonCode, fact.Reason = "blocked", "freight_amount_overflow", "运费实账合计超出安全整数范围"
			}
		}
		result[charge.OrderID] = fact
	}
	for orderID, fact := range result {
		switch {
		case fact.Status == "blocked" || fact.Status == "mismatch":
		case fact.BilledShipmentCount == 0:
			fact.Status = "missing"
		case fact.ShipmentCount == 0 || fact.BilledShipmentCount > fact.ShipmentCount:
			fact.Status, fact.ReasonCode, fact.Reason = "blocked", "freight_shipment_coverage_invalid", "运费账单关联的包裹与订单当前包裹数量不一致"
		case fact.BilledShipmentCount < fact.ShipmentCount:
			fact.Status, fact.ReasonCode, fact.Reason = "pending", "freight_billing_incomplete", "订单仍有包裹未形成承运商运费实账"
		default:
			fact.Status = "confirmed"
		}
		result[orderID] = fact
	}
	return result, nil
}

func appendUniqueID(ids []uuid.UUID, id uuid.UUID) []uuid.UUID {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func pagesOf(total int64, pageSize int) int {
	if total == 0 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}

func sortedUUIDs(values []uuid.UUID) []uuid.UUID {
	result := append([]uuid.UUID(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

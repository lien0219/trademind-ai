package warehousefee

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidInput       = errors.New("invalid warehouse fee input")
	ErrNotFound           = errors.New("warehouse fee record not found")
	ErrConflict           = errors.New("warehouse fee revision, calculation, or idempotency conflict")
	ErrFulfillmentBlocked = errors.New("warehouse fee fulfillment facts are incomplete")
	rateCodePattern       = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
)

type Service struct {
	DB          *gorm.DB
	Fulfillment FulfillmentReader
	Clock       func() time.Time
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func normalizeRateCode(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }
func normalizeStatus(value string) string   { return strings.ToLower(strings.TrimSpace(value)) }
func normalizeCurrency(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func validFee(value int64) bool { return value >= 0 && value <= MaxJSONSafeInteger }

func validateRateValues(name, currency string, outbound, picking, packing int64) error {
	if strings.TrimSpace(name) == "" || len([]rune(strings.TrimSpace(name))) > 160 || !validCurrency(normalizeCurrency(currency)) || !validFee(outbound) || !validFee(picking) || !validFee(packing) {
		return ErrInvalidInput
	}
	return nil
}

func actorPointer(actor *uuid.UUID) *uuid.UUID {
	if actor == nil || *actor == uuid.Nil {
		return nil
	}
	value := *actor
	return &value
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") || strings.Contains(message, "unique constraint") || strings.Contains(message, "unique violation") || strings.Contains(message, "constraint failed") || strings.Contains(message, "sqlstate 23505") || strings.Contains(message, "error 1062")
}

func (s *Service) ensureWarehouse(ctx context.Context, db *gorm.DB, tenantID int64, warehouseID uuid.UUID) error {
	var count int64
	if err := db.WithContext(ctx).Model(&warehouse.Warehouse{}).Where("tenant_id = ? AND id = ?", tenantID, warehouseID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return ErrNotFound
	}
	return nil
}

func rateRevisionFromCard(card RateCard, actor *uuid.UUID) RateCardRevision {
	return RateCardRevision{
		TenantID: card.TenantID, RateCardID: card.ID, Revision: card.Revision, Name: card.Name,
		Currency: card.Currency, OutboundBaseFeeMinor: card.OutboundBaseFeeMinor,
		PickingFeePerItemMinor:    card.PickingFeePerItemMinor,
		PackingFeePerPackageMinor: card.PackingFeePerPackageMinor,
		Status:                    card.Status, CreatedBy: actorPointer(actor),
	}
}

func (s *Service) CreateRateCard(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateRateCardInput) (*RateCard, error) {
	if s == nil || s.DB == nil || tenantID < 0 || in.WarehouseID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	code, name, currency := normalizeRateCode(in.Code), strings.TrimSpace(in.Name), normalizeCurrency(in.Currency)
	if !rateCodePattern.MatchString(code) || validateRateValues(name, currency, in.OutboundBaseFeeMinor, in.PickingFeePerItemMinor, in.PackingFeePerPackageMinor) != nil {
		return nil, ErrInvalidInput
	}
	card := RateCard{
		TenantID: tenantID, WarehouseID: in.WarehouseID, Code: code, Name: name, Currency: currency,
		OutboundBaseFeeMinor: in.OutboundBaseFeeMinor, PickingFeePerItemMinor: in.PickingFeePerItemMinor,
		PackingFeePerPackageMinor: in.PackingFeePerPackageMinor, Status: StatusActive, Revision: 1,
		CreatedBy: actorPointer(actor), UpdatedBy: actorPointer(actor),
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.ensureWarehouse(ctx, tx, tenantID, in.WarehouseID); err != nil {
			return err
		}
		if err := tx.Create(&card).Error; err != nil {
			return err
		}
		revision := rateRevisionFromCard(card, actor)
		return tx.Create(&revision).Error
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return s.GetRateCard(ctx, tenantID, card.ID)
}

func (s *Service) UpdateRateCard(ctx context.Context, tenantID int64, actor *uuid.UUID, id uuid.UUID, in UpdateRateCardInput) (*RateCard, error) {
	status, name, currency := normalizeStatus(in.Status), strings.TrimSpace(in.Name), normalizeCurrency(in.Currency)
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil || in.ExpectedRevision < 1 || (status != StatusActive && status != StatusInactive) || validateRateValues(name, currency, in.OutboundBaseFeeMinor, in.PickingFeePerItemMinor, in.PackingFeePerPackageMinor) != nil {
		return nil, ErrInvalidInput
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var card RateCard
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&card).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if card.Revision != in.ExpectedRevision {
			return ErrConflict
		}
		card.Name, card.Currency, card.Status = name, currency, status
		card.OutboundBaseFeeMinor, card.PickingFeePerItemMinor, card.PackingFeePerPackageMinor = in.OutboundBaseFeeMinor, in.PickingFeePerItemMinor, in.PackingFeePerPackageMinor
		card.Revision++
		card.UpdatedBy = actorPointer(actor)
		result := tx.Model(&RateCard{}).Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, id, in.ExpectedRevision).Updates(map[string]any{
			"name": card.Name, "currency": card.Currency, "outbound_base_fee_minor": card.OutboundBaseFeeMinor,
			"picking_fee_per_item_minor": card.PickingFeePerItemMinor, "packing_fee_per_package_minor": card.PackingFeePerPackageMinor,
			"status": card.Status, "revision": card.Revision, "updated_by": card.UpdatedBy,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConflict
		}
		revision := rateRevisionFromCard(card, actor)
		return tx.Create(&revision).Error
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return s.GetRateCard(ctx, tenantID, id)
}

type rateCardJoined struct {
	RateCard
	JoinedWarehouseCode string `gorm:"column:joined_warehouse_code"`
	JoinedWarehouseName string `gorm:"column:joined_warehouse_name"`
}

func selectRateCards(db *gorm.DB) *gorm.DB {
	return db.Table("warehouse_fee_rate_cards AS rate").
		Select("rate.*, warehouse.code AS joined_warehouse_code, warehouse.name AS joined_warehouse_name").
		Joins("JOIN warehouses AS warehouse ON warehouse.id = rate.warehouse_id AND warehouse.tenant_id = rate.tenant_id AND warehouse.deleted_at IS NULL")
}

func rateCardFromJoined(row rateCardJoined) RateCard {
	card := row.RateCard
	card.WarehouseCode, card.WarehouseName = row.JoinedWarehouseCode, row.JoinedWarehouseName
	return card
}

func (s *Service) ListRateCards(ctx context.Context, tenantID int64, warehouseID *uuid.UUID, includeInactive bool) ([]RateCard, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalidInput
	}
	q := selectRateCards(s.DB.WithContext(ctx)).Where("rate.tenant_id = ? AND rate.deleted_at IS NULL", tenantID)
	if warehouseID != nil && *warehouseID != uuid.Nil {
		q = q.Where("rate.warehouse_id = ?", *warehouseID)
	}
	if !includeInactive {
		q = q.Where("rate.status = ?", StatusActive)
	}
	var joined []rateCardJoined
	if err := q.Order("warehouse.code ASC, rate.code ASC, rate.id ASC").Scan(&joined).Error; err != nil {
		return nil, err
	}
	rows := make([]RateCard, 0, len(joined))
	for _, row := range joined {
		rows = append(rows, rateCardFromJoined(row))
	}
	return rows, nil
}

func (s *Service) GetRateCard(ctx context.Context, tenantID int64, id uuid.UUID) (*RateCard, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var joined rateCardJoined
	if err := selectRateCards(s.DB.WithContext(ctx)).Where("rate.tenant_id = ? AND rate.id = ? AND rate.deleted_at IS NULL", tenantID, id).Take(&joined).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	card := rateCardFromJoined(joined)
	return &card, nil
}

func (s *Service) GetRateCardDetail(ctx context.Context, tenantID int64, id uuid.UUID) (*RateCardDetail, error) {
	card, err := s.GetRateCard(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	revisions := make([]RateCardRevision, 0)
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND rate_card_id = ?", tenantID, id).Order("revision DESC, id DESC").Find(&revisions).Error; err != nil {
		return nil, err
	}
	return &RateCardDetail{RateCard: *card, Revisions: revisions}, nil
}

func (s *Service) rateAtRevision(ctx context.Context, db *gorm.DB, tenantID int64, id uuid.UUID, revision int, requireCurrent bool) (*RateCard, *RateCardRevision, error) {
	var card RateCard
	q := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id)
	if requireCurrent {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.First(&card).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if revision < 1 || (requireCurrent && (card.Revision != revision || card.Status != StatusActive)) {
		return nil, nil, ErrConflict
	}
	var rateRevision RateCardRevision
	if err := db.WithContext(ctx).Where("tenant_id = ? AND rate_card_id = ? AND revision = ?", tenantID, id, revision).Take(&rateRevision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrConflict
		}
		return nil, nil, err
	}
	return &card, &rateRevision, nil
}

type calculationFacts struct {
	OrderID                   uuid.UUID `json:"orderId"`
	OrderCurrency             string    `json:"orderCurrency"`
	WarehouseID               uuid.UUID `json:"warehouseId"`
	WaveID                    uuid.UUID `json:"waveId"`
	WaveRevision              int       `json:"waveRevision"`
	WaveOrderID               uuid.UUID `json:"waveOrderId"`
	PackVerificationID        uuid.UUID `json:"packVerificationId"`
	PackageCode               string    `json:"packageCode"`
	ItemQuantity              int       `json:"itemQuantity"`
	PackageQuantity           int       `json:"packageQuantity"`
	RateCardID                uuid.UUID `json:"rateCardId"`
	RateCardRevision          int       `json:"rateCardRevision"`
	Currency                  string    `json:"currency"`
	OutboundBaseFeeMinor      int64     `json:"outboundBaseFeeMinor"`
	PickingFeePerItemMinor    int64     `json:"pickingFeePerItemMinor"`
	PackingFeePerPackageMinor int64     `json:"packingFeePerPackageMinor"`
}

func hashValue(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func safeMultiply(left int64, right int) (int64, bool) {
	if left < 0 || right < 0 || (right > 0 && left > math.MaxInt64/int64(right)) {
		return 0, false
	}
	value := left * int64(right)
	return value, value <= MaxJSONSafeInteger
}

func safeAdd(left, right int64) (int64, bool) {
	if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
		return 0, false
	}
	value := left + right
	return value, value >= -MaxJSONSafeInteger && value <= MaxJSONSafeInteger
}

func buildPreview(fact *order.WarehouseFeeFact, card *RateCard, revision *RateCardRevision) (*Preview, error) {
	if fact == nil || card == nil || revision == nil || fact.OrderID == uuid.Nil || fact.WarehouseID == uuid.Nil || fact.WaveID == uuid.Nil || fact.WaveOrderID == uuid.Nil || fact.PackVerificationID == nil || *fact.PackVerificationID == uuid.Nil || strings.TrimSpace(fact.PackageCode) == "" || fact.ItemQuantity < 1 || fact.PackageQuantity != 1 || fact.WaveRevision < 1 || (fact.WaveStatus != order.FulfillmentWaveCompleted && fact.WaveStatus != order.FulfillmentWavePartial) || fact.WaveOrderStatus != order.FulfillmentWaveOrderFulfilled {
		return nil, ErrFulfillmentBlocked
	}
	if card.WarehouseID != fact.WarehouseID || revision.Revision != card.Revision || revision.Status != StatusActive {
		return nil, ErrConflict
	}
	currency := normalizeCurrency(revision.Currency)
	orderCurrency := normalizeCurrency(fact.Currency)
	if !validCurrency(currency) || !validCurrency(orderCurrency) || currency != orderCurrency ||
		!validFee(revision.OutboundBaseFeeMinor) || !validFee(revision.PickingFeePerItemMinor) || !validFee(revision.PackingFeePerPackageMinor) {
		return nil, ErrConflict
	}
	picking, ok := safeMultiply(revision.PickingFeePerItemMinor, fact.ItemQuantity)
	if !ok {
		return nil, ErrInvalidInput
	}
	packing, ok := safeMultiply(revision.PackingFeePerPackageMinor, fact.PackageQuantity)
	if !ok {
		return nil, ErrInvalidInput
	}
	amount, ok := safeAdd(revision.OutboundBaseFeeMinor, picking)
	if !ok {
		return nil, ErrInvalidInput
	}
	amount, ok = safeAdd(amount, packing)
	if !ok || amount < 0 {
		return nil, ErrInvalidInput
	}
	facts := calculationFacts{
		OrderID: fact.OrderID, OrderCurrency: orderCurrency, WarehouseID: fact.WarehouseID, WaveID: fact.WaveID, WaveRevision: fact.WaveRevision,
		WaveOrderID: fact.WaveOrderID, PackVerificationID: *fact.PackVerificationID,
		PackageCode: strings.TrimSpace(fact.PackageCode), ItemQuantity: fact.ItemQuantity, PackageQuantity: fact.PackageQuantity,
		RateCardID: card.ID, RateCardRevision: revision.Revision, Currency: currency,
		OutboundBaseFeeMinor: revision.OutboundBaseFeeMinor, PickingFeePerItemMinor: revision.PickingFeePerItemMinor,
		PackingFeePerPackageMinor: revision.PackingFeePerPackageMinor,
	}
	hash, err := hashValue(facts)
	if err != nil {
		return nil, err
	}
	return &Preview{
		OrderID: fact.OrderID, OrderNo: fact.OrderNo, ShopID: fact.ShopID,
		WarehouseID: fact.WarehouseID, WarehouseCode: fact.WarehouseCode, WarehouseName: fact.WarehouseName,
		WaveID: fact.WaveID, WaveNo: fact.WaveNo, WaveRevision: fact.WaveRevision, WaveOrderID: fact.WaveOrderID,
		PackVerificationID: *fact.PackVerificationID, PackageCode: strings.TrimSpace(fact.PackageCode),
		ItemQuantity: fact.ItemQuantity, PackageQuantity: fact.PackageQuantity,
		RateCardID: card.ID, RateCardCode: card.Code, RateCardName: revision.Name, RateCardRevision: revision.Revision,
		OutboundBaseFeeMinor: revision.OutboundBaseFeeMinor, PickingFeePerItemMinor: revision.PickingFeePerItemMinor,
		PickingFeeMinor: picking, PackingFeePerPackageMinor: revision.PackingFeePerPackageMinor, PackingFeeMinor: packing,
		AmountMinor: amount, Currency: currency, CalculationHash: hash,
	}, nil
}

func (s *Service) previewWithDB(ctx context.Context, db *gorm.DB, tenantID int64, scope order.WarehouseFeeScope, in PreviewInput, lock bool) (*Preview, error) {
	if s == nil || db == nil || s.Fulfillment == nil || tenantID < 0 || in.OrderID == uuid.Nil || in.RateCardID == uuid.Nil || in.RateCardRevision < 1 {
		return nil, ErrInvalidInput
	}
	card, revision, err := s.rateAtRevision(ctx, db, tenantID, in.RateCardID, in.RateCardRevision, lock)
	if err != nil {
		return nil, err
	}
	if card.Status != StatusActive || card.Revision != in.RateCardRevision {
		return nil, ErrConflict
	}
	fact, err := s.Fulfillment.GetWarehouseFeeFact(ctx, db, tenantID, scope, in.OrderID, lock)
	if err != nil {
		if errors.Is(err, order.ErrFulfillmentWaveNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return buildPreview(fact, card, revision)
}

func (s *Service) Preview(ctx context.Context, tenantID int64, scope order.WarehouseFeeScope, in PreviewInput) (*Preview, error) {
	if s == nil || s.DB == nil {
		return nil, ErrInvalidInput
	}
	return s.previewWithDB(ctx, s.DB, tenantID, scope, in, false)
}

func normalizeKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) < 8 || len(value) > 128 {
		return "", ErrInvalidInput
	}
	return value, nil
}

func (s *Service) Confirm(ctx context.Context, tenantID int64, scope order.WarehouseFeeScope, actor *uuid.UUID, in ConfirmInput) (*ConfirmResult, error) {
	key, err := normalizeKey(in.IdempotencyKey)
	expectedHash := strings.ToLower(strings.TrimSpace(in.CalculationHash))
	if s == nil || s.DB == nil || err != nil || len(expectedHash) != 64 || in.WaveRevision < 1 {
		return nil, ErrInvalidInput
	}
	requestHash, err := hashValue(struct {
		OrderID          uuid.UUID `json:"orderId"`
		RateCardID       uuid.UUID `json:"rateCardId"`
		RateCardRevision int       `json:"rateCardRevision"`
		WaveRevision     int       `json:"waveRevision"`
		CalculationHash  string    `json:"calculationHash"`
	}{in.OrderID, in.RateCardID, in.RateCardRevision, in.WaveRevision, expectedHash})
	if err != nil {
		return nil, err
	}
	var result ConfirmResult
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing Snapshot
		findErr := applySnapshotScope(tx.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key), scope).Take(&existing).Error
		if findErr == nil {
			if existing.RequestHash != requestHash {
				return ErrConflict
			}
			result = ConfirmResult{Snapshot: existing, Replayed: true}
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		preview, previewErr := s.previewWithDB(ctx, tx, tenantID, scope, PreviewInput{OrderID: in.OrderID, RateCardID: in.RateCardID, RateCardRevision: in.RateCardRevision}, true)
		if previewErr != nil {
			return previewErr
		}
		if preview.WaveRevision != in.WaveRevision || preview.CalculationHash != expectedHash {
			return ErrConflict
		}
		findErr = applySnapshotScope(tx.Where("tenant_id = ? AND order_id = ?", tenantID, in.OrderID), scope).Take(&existing).Error
		if findErr == nil {
			if existing.CalculationHash != expectedHash {
				return ErrConflict
			}
			result = ConfirmResult{Snapshot: existing, Replayed: true}
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		now := s.now()
		snapshot := Snapshot{
			TenantID: tenantID, OrderID: preview.OrderID, ShopID: preview.ShopID, OrderNo: preview.OrderNo,
			WarehouseID: preview.WarehouseID, WarehouseCode: preview.WarehouseCode, WarehouseName: preview.WarehouseName,
			WaveID: preview.WaveID, WaveNo: preview.WaveNo, WaveRevision: preview.WaveRevision, WaveOrderID: preview.WaveOrderID,
			PackVerificationID: preview.PackVerificationID, PackageCode: preview.PackageCode,
			ItemQuantity: preview.ItemQuantity, PackageQuantity: preview.PackageQuantity,
			RateCardID: preview.RateCardID, RateCardRevision: preview.RateCardRevision, RateCardCode: preview.RateCardCode, RateCardName: preview.RateCardName,
			OutboundBaseFeeMinor: preview.OutboundBaseFeeMinor, PickingFeePerItemMinor: preview.PickingFeePerItemMinor,
			PickingFeeMinor: preview.PickingFeeMinor, PackingFeePerPackageMinor: preview.PackingFeePerPackageMinor,
			PackingFeeMinor: preview.PackingFeeMinor, AmountMinor: preview.AmountMinor, Currency: preview.Currency,
			CalculationHash: expectedHash, IdempotencyKey: key, RequestHash: requestHash, ConfirmedBy: actorPointer(actor), ConfirmedAt: now,
		}
		if err := tx.Create(&snapshot).Error; err != nil {
			return err
		}
		result = ConfirmResult{Snapshot: snapshot}
		return nil
	})
	if err == nil {
		return &result, nil
	}
	if isUniqueViolation(err) {
		var existing Snapshot
		if findErr := applySnapshotScope(s.DB.WithContext(ctx).Where("tenant_id = ? AND (idempotency_key = ? OR order_id = ?)", tenantID, key, in.OrderID), scope).Order("created_at ASC").Take(&existing).Error; findErr == nil && existing.CalculationHash == expectedHash {
			return &ConfirmResult{Snapshot: existing, Replayed: true}, nil
		}
		return nil, ErrConflict
	}
	return nil, err
}

func normalizeSnapshotListQuery(in SnapshotListQuery) (SnapshotListQuery, error) {
	in.OrderNo = strings.TrimSpace(in.OrderNo)
	in.Currency = normalizeCurrency(in.Currency)
	if in.Page < 1 {
		in.Page = 1
	}
	if in.PageSize < 1 {
		in.PageSize = 20
	}
	if in.PageSize > MaxPageSize || (in.Currency != "" && !validCurrency(in.Currency)) {
		return in, ErrInvalidInput
	}
	return in, nil
}

func applySnapshotScope(q *gorm.DB, scope order.WarehouseFeeScope) *gorm.DB {
	if !scope.RestrictStoreScope {
		return q
	}
	if len(scope.AllowedShopIDs) == 0 {
		return q.Where("1 = 0")
	}
	return q.Where("shop_id IN ?", scope.AllowedShopIDs)
}

func summarizeSnapshot(snapshot Snapshot, adjustments []Adjustment) (SnapshotSummary, error) {
	result := SnapshotSummary{Snapshot: snapshot, NetAmountMinor: snapshot.AmountMinor}
	if snapshot.ID == uuid.Nil || snapshot.OrderID == uuid.Nil || snapshot.WarehouseID == uuid.Nil || snapshot.WaveID == uuid.Nil || snapshot.WaveOrderID == uuid.Nil || snapshot.PackVerificationID == uuid.Nil || snapshot.RateCardID == uuid.Nil || snapshot.ItemQuantity < 1 || snapshot.PackageQuantity != 1 || snapshot.RateCardRevision < 1 || snapshot.WaveRevision < 1 || !validCurrency(normalizeCurrency(snapshot.Currency)) || !validFee(snapshot.OutboundBaseFeeMinor) || !validFee(snapshot.PickingFeePerItemMinor) || !validFee(snapshot.PickingFeeMinor) || !validFee(snapshot.PackingFeePerPackageMinor) || !validFee(snapshot.PackingFeeMinor) || !validFee(snapshot.AmountMinor) {
		return result, ErrConflict
	}
	baseAndPicking, ok := safeAdd(snapshot.OutboundBaseFeeMinor, snapshot.PickingFeeMinor)
	if !ok {
		return result, ErrConflict
	}
	expectedAmount, ok := safeAdd(baseAndPicking, snapshot.PackingFeeMinor)
	if !ok || expectedAmount != snapshot.AmountMinor {
		return result, ErrConflict
	}
	byID := make(map[uuid.UUID]Adjustment, len(adjustments))
	for _, adjustment := range adjustments {
		byID[adjustment.ID] = adjustment
	}
	for _, adjustment := range adjustments {
		if adjustment.ID == uuid.Nil || adjustment.SnapshotID != snapshot.ID || adjustment.OrderID != snapshot.OrderID || adjustment.AmountMinor == 0 || adjustment.AmountMinor < -MaxJSONSafeInteger || adjustment.AmountMinor > MaxJSONSafeInteger || normalizeCurrency(adjustment.Currency) != normalizeCurrency(snapshot.Currency) {
			return result, ErrConflict
		}
		switch adjustment.FactType {
		case FactAdjustment:
			if adjustment.ReversesAdjustmentID != nil {
				return result, ErrConflict
			}
		case FactReversal:
			if adjustment.ReversesAdjustmentID == nil {
				return result, ErrConflict
			}
			target, exists := byID[*adjustment.ReversesAdjustmentID]
			if !exists || target.FactType != FactAdjustment || target.ReversesAdjustmentID != nil || target.AmountMinor == math.MinInt64 || adjustment.AmountMinor != -target.AmountMinor {
				return result, ErrConflict
			}
		default:
			return result, ErrConflict
		}
		next, ok := safeAdd(result.AdjustmentMinor, adjustment.AmountMinor)
		if !ok {
			return result, ErrConflict
		}
		result.AdjustmentMinor = next
		result.AdjustmentCount++
	}
	net, ok := safeAdd(snapshot.AmountMinor, result.AdjustmentMinor)
	if !ok || net < 0 {
		return result, ErrConflict
	}
	result.NetAmountMinor = net
	return result, nil
}

func (s *Service) ListSnapshots(ctx context.Context, tenantID int64, scope order.WarehouseFeeScope, raw SnapshotListQuery) (*SnapshotList, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalidInput
	}
	in, err := normalizeSnapshotListQuery(raw)
	if err != nil {
		return nil, err
	}
	q := applySnapshotScope(s.DB.WithContext(ctx).Model(&Snapshot{}).Where("tenant_id = ?", tenantID), scope)
	if in.OrderNo != "" {
		q = q.Where("LOWER(order_no) LIKE ?", "%"+strings.ToLower(in.OrderNo)+"%")
	}
	if in.WarehouseID != nil && *in.WarehouseID != uuid.Nil {
		q = q.Where("warehouse_id = ?", *in.WarehouseID)
	}
	if in.Currency != "" {
		q = q.Where("currency = ?", in.Currency)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	snapshots := make([]Snapshot, 0)
	if total > 0 {
		if err := q.Order("confirmed_at DESC, id DESC").Offset((in.Page - 1) * in.PageSize).Limit(in.PageSize).Find(&snapshots).Error; err != nil {
			return nil, err
		}
	}
	adjustmentsBySnapshot, err := s.adjustmentsForSnapshots(ctx, tenantID, snapshotIDs(snapshots))
	if err != nil {
		return nil, err
	}
	rows := make([]SnapshotSummary, 0, len(snapshots))
	for _, snapshot := range snapshots {
		summary, err := summarizeSnapshot(snapshot, adjustmentsBySnapshot[snapshot.ID])
		if err != nil {
			return nil, err
		}
		rows = append(rows, summary)
	}
	return &SnapshotList{List: rows, Page: in.Page, PageSize: in.PageSize, Total: total, TotalPages: int((total + int64(in.PageSize) - 1) / int64(in.PageSize))}, nil
}

func snapshotIDs(rows []Snapshot) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func (s *Service) adjustmentsForSnapshots(ctx context.Context, tenantID int64, ids []uuid.UUID) (map[uuid.UUID][]Adjustment, error) {
	result := make(map[uuid.UUID][]Adjustment)
	if len(ids) == 0 {
		return result, nil
	}
	var rows []Adjustment
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND snapshot_id IN ?", tenantID, ids).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.SnapshotID] = append(result[row.SnapshotID], row)
	}
	return result, nil
}

func (s *Service) GetSnapshot(ctx context.Context, tenantID int64, scope order.WarehouseFeeScope, id uuid.UUID) (*SnapshotDetail, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var snapshot Snapshot
	if err := applySnapshotScope(s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id), scope).Take(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	grouped, err := s.adjustmentsForSnapshots(ctx, tenantID, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	summary, err := summarizeSnapshot(snapshot, grouped[id])
	if err != nil {
		return nil, err
	}
	return &SnapshotDetail{SnapshotSummary: summary, Adjustments: grouped[id]}, nil
}

func (s *Service) ListCandidates(ctx context.Context, tenantID int64, scope order.WarehouseFeeScope, in order.WarehouseFeeFactQuery) (*CandidateList, error) {
	if s == nil || s.DB == nil || s.Fulfillment == nil || tenantID < 0 {
		return nil, ErrInvalidInput
	}
	facts, err := s.Fulfillment.ListWarehouseFeeFacts(ctx, s.DB, tenantID, scope, in)
	if err != nil {
		return nil, err
	}
	orderIDs := make([]uuid.UUID, 0, len(facts.List))
	for _, fact := range facts.List {
		orderIDs = append(orderIDs, fact.OrderID)
	}
	confirmed := make(map[uuid.UUID]uuid.UUID)
	if len(orderIDs) > 0 {
		var rows []Snapshot
		if err := s.DB.WithContext(ctx).Select("id, order_id").Where("tenant_id = ? AND order_id IN ?", tenantID, orderIDs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			confirmed[row.OrderID] = row.ID
		}
	}
	list := make([]Candidate, 0, len(facts.List))
	for _, fact := range facts.List {
		candidate := Candidate{WarehouseFeeFact: fact}
		if id, ok := confirmed[fact.OrderID]; ok {
			candidate.SnapshotID, candidate.Confirmed = &id, true
		}
		list = append(list, candidate)
	}
	return &CandidateList{List: list, Page: facts.Page, PageSize: facts.PageSize, Total: facts.Total, TotalPages: facts.TotalPages}, nil
}

func adjustmentRequestHash(snapshotID uuid.UUID, factType string, amount int64, reason, key string, reverses *uuid.UUID) (string, error) {
	return hashValue(struct {
		SnapshotID uuid.UUID  `json:"snapshotId"`
		FactType   string     `json:"factType"`
		Amount     int64      `json:"amountMinor"`
		Reason     string     `json:"reason"`
		Reverses   *uuid.UUID `json:"reversesAdjustmentId,omitempty"`
		Key        string     `json:"idempotencyKey"`
	}{snapshotID, factType, amount, reason, reverses, key})
}

func (s *Service) adjustmentReplay(tx *gorm.DB, tenantID int64, key, requestHash string) (*AdjustmentResult, error) {
	var existing Adjustment
	if err := tx.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).Take(&existing).Error; err != nil {
		return nil, err
	}
	if existing.RequestHash != requestHash {
		return nil, ErrConflict
	}
	return &AdjustmentResult{Adjustment: existing, Replayed: true}, nil
}

func (s *Service) lockedSnapshot(ctx context.Context, tx *gorm.DB, tenantID int64, scope order.WarehouseFeeScope, id uuid.UUID) (*Snapshot, error) {
	var snapshot Snapshot
	q := applySnapshotScope(tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id), scope)
	if err := q.Take(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &snapshot, nil
}

func (s *Service) validateNetAmount(ctx context.Context, tx *gorm.DB, snapshot Snapshot, delta int64) error {
	var current int64
	if err := tx.WithContext(ctx).Model(&Adjustment{}).Where("tenant_id = ? AND snapshot_id = ?", snapshot.TenantID, snapshot.ID).Select("COALESCE(SUM(amount_minor), 0)").Scan(&current).Error; err != nil {
		return err
	}
	adjusted, ok := safeAdd(current, delta)
	if !ok {
		return ErrInvalidInput
	}
	net, ok := safeAdd(snapshot.AmountMinor, adjusted)
	if !ok || net < 0 {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) CreateAdjustment(ctx context.Context, tenantID int64, scope order.WarehouseFeeScope, actor *uuid.UUID, snapshotID uuid.UUID, in CreateAdjustmentInput) (*AdjustmentResult, error) {
	key, keyErr := normalizeKey(in.IdempotencyKey)
	reason := strings.TrimSpace(in.Reason)
	if s == nil || s.DB == nil || tenantID < 0 || snapshotID == uuid.Nil || keyErr != nil || in.AmountMinor == 0 || in.AmountMinor < -MaxJSONSafeInteger || in.AmountMinor > MaxJSONSafeInteger || reason == "" || len([]rune(reason)) > 500 {
		return nil, ErrInvalidInput
	}
	requestHash, err := adjustmentRequestHash(snapshotID, FactAdjustment, in.AmountMinor, reason, key, nil)
	if err != nil {
		return nil, err
	}
	var result *AdjustmentResult
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		snapshot, err := s.lockedSnapshot(ctx, tx, tenantID, scope, snapshotID)
		if err != nil {
			return err
		}
		if replay, replayErr := s.adjustmentReplay(tx, tenantID, key, requestHash); replayErr == nil {
			result = replay
			return nil
		} else if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}
		if err := s.validateNetAmount(ctx, tx, *snapshot, in.AmountMinor); err != nil {
			return err
		}
		row := Adjustment{TenantID: tenantID, SnapshotID: snapshot.ID, OrderID: snapshot.OrderID, FactType: FactAdjustment, AmountMinor: in.AmountMinor, Currency: snapshot.Currency, Reason: reason, IdempotencyKey: key, RequestHash: requestHash, ActorID: actorPointer(actor)}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		result = &AdjustmentResult{Adjustment: row}
		return nil
	})
	if err == nil {
		return result, nil
	}
	if isUniqueViolation(err) {
		if replay, replayErr := s.adjustmentReplay(s.DB.WithContext(ctx), tenantID, key, requestHash); replayErr == nil {
			return replay, nil
		}
		return nil, ErrConflict
	}
	return nil, err
}

func (s *Service) ReverseAdjustment(ctx context.Context, tenantID int64, scope order.WarehouseFeeScope, actor *uuid.UUID, snapshotID, adjustmentID uuid.UUID, in ReverseAdjustmentInput) (*AdjustmentResult, error) {
	key, keyErr := normalizeKey(in.IdempotencyKey)
	reason := strings.TrimSpace(in.Reason)
	if s == nil || s.DB == nil || tenantID < 0 || snapshotID == uuid.Nil || adjustmentID == uuid.Nil || keyErr != nil || reason == "" || len([]rune(reason)) > 500 {
		return nil, ErrInvalidInput
	}
	var result *AdjustmentResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		snapshot, err := s.lockedSnapshot(ctx, tx, tenantID, scope, snapshotID)
		if err != nil {
			return err
		}
		var target Adjustment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND snapshot_id = ? AND id = ?", tenantID, snapshotID, adjustmentID).Take(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if target.FactType != FactAdjustment || target.AmountMinor == math.MinInt64 {
			return ErrInvalidInput
		}
		amount := -target.AmountMinor
		requestHash, err := adjustmentRequestHash(snapshotID, FactReversal, amount, reason, key, &target.ID)
		if err != nil {
			return err
		}
		if replay, replayErr := s.adjustmentReplay(tx, tenantID, key, requestHash); replayErr == nil {
			result = replay
			return nil
		} else if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}
		if err := s.validateNetAmount(ctx, tx, *snapshot, amount); err != nil {
			return err
		}
		row := Adjustment{TenantID: tenantID, SnapshotID: snapshot.ID, OrderID: snapshot.OrderID, FactType: FactReversal, AmountMinor: amount, Currency: snapshot.Currency, Reason: reason, ReversesAdjustmentID: &target.ID, IdempotencyKey: key, RequestHash: requestHash, ActorID: actorPointer(actor)}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		result = &AdjustmentResult{Adjustment: row}
		return nil
	})
	if err == nil {
		return result, nil
	}
	if isUniqueViolation(err) {
		return nil, ErrConflict
	}
	return nil, err
}

func (s *Service) ProfitabilityFeesForOrders(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID]ProfitabilityFact, error) {
	result := make(map[uuid.UUID]ProfitabilityFact)
	if s == nil || s.DB == nil || len(orderIDs) == 0 {
		return result, nil
	}
	var snapshots []Snapshot
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND order_id IN ?", tenantID, orderIDs).Order("confirmed_at ASC, id ASC").Find(&snapshots).Error; err != nil {
		return nil, err
	}
	grouped, err := s.adjustmentsForSnapshots(ctx, tenantID, snapshotIDs(snapshots))
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		fact := ProfitabilityFact{OrderID: snapshot.OrderID, SnapshotID: snapshot.ID, Currency: snapshot.Currency, Status: "confirmed", SourceAt: snapshot.ConfirmedAt}
		summary, summaryErr := summarizeSnapshot(snapshot, grouped[snapshot.ID])
		if summaryErr != nil {
			fact.Status, fact.ReasonCode, fact.Reason = "blocked", "warehouse_fee_facts_invalid", "仓库操作费事实无法安全汇总"
		} else {
			fact.AmountMinor = summary.NetAmountMinor
		}
		for _, adjustment := range grouped[snapshot.ID] {
			fact.AdjustmentIDs = append(fact.AdjustmentIDs, adjustment.ID)
			if adjustment.CreatedAt.After(fact.SourceAt) {
				fact.SourceAt = adjustment.CreatedAt
			}
		}
		if _, exists := result[snapshot.OrderID]; exists {
			fact.Status, fact.ReasonCode, fact.Reason = "blocked", "multiple_warehouse_fee_snapshots", "订单存在多条仓库操作费确认快照"
		}
		result[snapshot.OrderID] = fact
	}
	return result, nil
}

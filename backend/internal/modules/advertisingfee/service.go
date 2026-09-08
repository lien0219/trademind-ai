package advertisingfee

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	ordermodule "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidInput = errors.New("invalid advertising fee input")
	ErrNotFound     = errors.New("advertising fee record not found")
	ErrConflict     = errors.New("advertising fee source, calculation, or idempotency conflict")
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

func (s *Service) loadShop(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, lock bool) (*shop.Shop, *time.Location, error) {
	var row shop.Shop
	query := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, shopID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	timezone := strings.TrimSpace(row.Timezone)
	if timezone == "" {
		return nil, nil, ErrConflict
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, nil, ErrConflict
	}
	return &row, location, nil
}

type orderCalculationFact struct {
	OrderID         uuid.UUID  `json:"orderId"`
	OrderNo         string     `json:"orderNo"`
	Status          string     `json:"status"`
	PaymentStatus   string     `json:"paymentStatus"`
	Currency        string     `json:"currency"`
	PaidAt          *time.Time `json:"paidAt,omitempty"`
	OrderedAt       *time.Time `json:"orderedAt,omitempty"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	Included        bool       `json:"included"`
	AllocatedMinor  *int64     `json:"allocatedMinor,omitempty"`
	ExclusionReason string     `json:"exclusionReason,omitempty"`
}

type spendCalculationFact struct {
	Line               int                    `json:"line"`
	SpendDate          string                 `json:"spendDate"`
	Currency           string                 `json:"currency"`
	SpendMinor         int64                  `json:"spendMinor"`
	SettlementCoverage string                 `json:"settlementCoverage"`
	RowHash            string                 `json:"rowHash"`
	Orders             []orderCalculationFact `json:"orders"`
}

func localDayBounds(date string, location *time.Location) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil || start.Format("2006-01-02") != date {
		return time.Time{}, time.Time{}, ErrInvalidInput
	}
	return start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}

func (s *Service) dayOrders(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, date string, location *time.Location, lock bool) ([]ordermodule.Order, error) {
	start, end, err := localDayBounds(date, location)
	if err != nil {
		return nil, err
	}
	query := db.WithContext(ctx).Where(
		"tenant_id = ? AND shop_id = ? AND ((paid_at >= ? AND paid_at < ?) OR (paid_at IS NULL AND ordered_at >= ? AND ordered_at < ?))",
		tenantID, shopID, start, end, start, end,
	)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	rows := make([]ordermodule.Order, 0)
	if err := query.Order("COALESCE(paid_at, ordered_at) ASC, order_no ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func classifyOrder(row ordermodule.Order, spend CSVRow, location *time.Location, attributed map[uuid.UUID]struct{}) (bool, string, string) {
	if _, exists := attributed[row.ID]; exists {
		return false, "order_already_attributed", "订单已有广告费用归属事实"
	}
	if strings.EqualFold(strings.TrimSpace(row.Status), ordermodule.StatusCancelled) {
		return false, "order_cancelled", "订单已取消"
	}
	paymentStatus := strings.ToLower(strings.TrimSpace(row.PaymentStatus))
	if row.PaidAt == nil || (paymentStatus != ordermodule.PaymentPaid && paymentStatus != ordermodule.PaymentPartiallyRefunded) {
		return false, "order_not_paid", "订单未处于已支付状态"
	}
	if row.PaidAt.In(location).Format("2006-01-02") != spend.SpendDate {
		return false, "paid_date_mismatch", "支付时间不属于费用日期"
	}
	if strings.ToUpper(strings.TrimSpace(row.Currency)) != spend.Currency {
		return false, "order_currency_mismatch", "订单币种与广告费用币种不一致"
	}
	return true, "", ""
}

func existingSpendKey(date, currency string) string { return date + "\x1f" + currency }

func (s *Service) existingSpends(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, rows []CSVRow, lock bool) (map[string]Spend, error) {
	result := make(map[string]Spend)
	if len(rows) == 0 {
		return result, nil
	}
	dates := make([]string, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, row.SpendDate)
	}
	query := db.WithContext(ctx).Where("tenant_id = ? AND shop_id = ? AND spend_date IN ?", tenantID, shopID, dates)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var existing []Spend
	if err := query.Find(&existing).Error; err != nil {
		return nil, err
	}
	for _, row := range existing {
		result[existingSpendKey(row.SpendDate, row.Currency)] = row
	}
	return result, nil
}

func (s *Service) attributedOrders(ctx context.Context, db *gorm.DB, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID]struct{}, error) {
	result := make(map[uuid.UUID]struct{})
	if len(orderIDs) == 0 {
		return result, nil
	}
	var rows []Allocation
	if err := db.WithContext(ctx).Select("order_id").Where("tenant_id = ? AND order_id IN ?", tenantID, orderIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.OrderID] = struct{}{}
	}
	return result, nil
}

func allocate(spend int64, count int, index int) int64 {
	base := spend / int64(count)
	if int64(index) < spend%int64(count) {
		return base + 1
	}
	return base
}

func (s *Service) previewWithDB(ctx context.Context, db *gorm.DB, tenantID int64, scope Scope, shopID uuid.UUID, fileName string, data []byte, lock bool) (*PreviewResult, error) {
	if s == nil || db == nil || tenantID < 0 || shopID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if !scopeAllowsShop(scope, shopID) {
		return nil, ErrNotFound
	}
	shopRow, location, err := s.loadShop(ctx, db, tenantID, shopID, lock)
	if err != nil {
		return nil, err
	}
	rows, issues := parseCSV(data)
	result := &PreviewResult{
		FileName: strings.TrimSpace(fileName), FileHash: fileHash(data), PolicyVersion: PolicyVersion,
		ShopID: shopID, ShopName: shopRow.ShopName, Platform: shopRow.Platform, ShopTimezone: shopRow.Timezone,
		SourceRows: len(rows) + invalidDataRowCount(issues), Rows: make([]SpendPreview, 0, len(rows)),
		Orders: []PreviewOrder{}, Issues: issues, Warnings: []ValidationIssue{},
	}
	var priorImport Import
	priorImportErr := db.WithContext(ctx).Where("tenant_id = ? AND shop_id = ? AND file_hash = ?", tenantID, shopID, result.FileHash).Take(&priorImport).Error
	if priorImportErr != nil && !errors.Is(priorImportErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("lookup prior advertising fee import: %w", priorImportErr)
	}
	priorImportFound := priorImportErr == nil
	existing, err := s.existingSpends(ctx, db, tenantID, shopID, rows, lock)
	if err != nil {
		return nil, err
	}
	calculationRows := make([]spendCalculationFact, 0, len(rows))
	for _, row := range rows {
		previewRow := SpendPreview{CSVRow: row}
		if found, ok := existing[existingSpendKey(row.SpendDate, row.Currency)]; ok {
			if found.RowHash != row.RowHash {
				result.Issues = append(result.Issues, ValidationIssue{Line: row.Line, Code: "spend_day_conflict", Message: "该店铺日期和币种已有不同广告费用事实"})
			} else {
				previewRow.Disposition = DispositionDup
				previewRow.EligibleOrderCount = found.EligibleOrderCount
				previewRow.ExcludedOrderCount = found.ExcludedOrderCount
				previewRow.CalculationHash = found.CalculationHash
				result.DuplicateSpends++
			}
			result.Rows = append(result.Rows, previewRow)
			continue
		}
		orders, queryErr := s.dayOrders(ctx, db, tenantID, shopID, row.SpendDate, location, lock)
		if queryErr != nil {
			return nil, queryErr
		}
		orderIDs := make([]uuid.UUID, 0, len(orders))
		for _, orderRow := range orders {
			orderIDs = append(orderIDs, orderRow.ID)
		}
		alreadyAttributed, queryErr := s.attributedOrders(ctx, db, tenantID, orderIDs)
		if queryErr != nil {
			return nil, queryErr
		}
		facts := make([]orderCalculationFact, 0, len(orders))
		eligibleIndexes := make([]int, 0, len(orders))
		for _, orderRow := range orders {
			included, reasonCode, reason := classifyOrder(orderRow, row, location, alreadyAttributed)
			fact := orderCalculationFact{
				OrderID: orderRow.ID, OrderNo: orderRow.OrderNo, Status: orderRow.Status,
				PaymentStatus: orderRow.PaymentStatus, Currency: strings.ToUpper(strings.TrimSpace(orderRow.Currency)),
				PaidAt: orderRow.PaidAt, OrderedAt: orderRow.OrderedAt, UpdatedAt: orderRow.UpdatedAt.UTC(),
				Included: included, ExclusionReason: reasonCode,
			}
			facts = append(facts, fact)
			if included {
				eligibleIndexes = append(eligibleIndexes, len(facts)-1)
			} else {
				result.Orders = append(result.Orders, PreviewOrder{
					Line: row.Line, SpendDate: row.SpendDate, OrderID: orderRow.ID, OrderNo: orderRow.OrderNo,
					PaidAt: orderRow.PaidAt, Currency: strings.ToUpper(strings.TrimSpace(orderRow.Currency)),
					Included: false, ReasonCode: reasonCode, Reason: reason,
				})
			}
		}
		if len(eligibleIndexes) == 0 {
			result.Issues = append(result.Issues, ValidationIssue{Line: row.Line, Code: "eligible_orders_missing", Message: "费用日期没有可归属的已支付未取消订单"})
		} else {
			for allocationIndex, factIndex := range eligibleIndexes {
				amount := allocate(row.SpendMinor, len(eligibleIndexes), allocationIndex)
				facts[factIndex].AllocatedMinor = &amount
				orderRow := facts[factIndex]
				result.Orders = append(result.Orders, PreviewOrder{
					Line: row.Line, SpendDate: row.SpendDate, OrderID: orderRow.OrderID, OrderNo: orderRow.OrderNo,
					PaidAt: orderRow.PaidAt, Currency: orderRow.Currency, AmountMinor: &amount, Included: true,
				})
			}
		}
		calculation := spendCalculationFact{
			Line: row.Line, SpendDate: row.SpendDate, Currency: row.Currency, SpendMinor: row.SpendMinor,
			SettlementCoverage: row.SettlementCoverage, RowHash: row.RowHash, Orders: facts,
		}
		calculationHash, hashErr := hashValue(calculation)
		if hashErr != nil {
			return nil, hashErr
		}
		previewRow.EligibleOrderCount = len(eligibleIndexes)
		previewRow.ExcludedOrderCount = len(orders) - len(eligibleIndexes)
		previewRow.CalculationHash = calculationHash
		result.NewSpends++
		result.AllocationCount += len(eligibleIndexes)
		result.Rows = append(result.Rows, previewRow)
		calculationRows = append(calculationRows, calculation)
		if row.SettlementCoverage == CoverageIncluded {
			result.Warnings = append(result.Warnings, ValidationIssue{Line: row.Line, Code: "advertising_in_platform_fee", Message: "广告费用声明已包含在平台费用中，将记录归属但不会重复计入利润"})
		} else if row.SettlementCoverage == CoverageUnknown {
			result.Warnings = append(result.Warnings, ValidationIssue{Line: row.Line, Code: "advertising_coverage_unknown", Message: "广告费用的结算覆盖未知，将记录归属但保持利润阻断"})
		}
	}
	calculationHash, err := hashValue(struct {
		FileHash     string                 `json:"fileHash"`
		ShopID       uuid.UUID              `json:"shopId"`
		ShopTimezone string                 `json:"shopTimezone"`
		Policy       string                 `json:"policy"`
		Rows         []spendCalculationFact `json:"rows"`
	}{result.FileHash, result.ShopID, result.ShopTimezone, PolicyVersion, calculationRows})
	if err != nil {
		return nil, err
	}
	if priorImportFound && result.NewSpends == 0 && result.DuplicateSpends == len(result.Rows) {
		calculationHash = priorImport.CalculationHash
	}
	result.CalculationHash = calculationHash
	result.Valid = len(result.Issues) == 0
	return result, nil
}

func (s *Service) Preview(ctx context.Context, tenantID int64, scope Scope, shopID uuid.UUID, fileName string, data []byte) (*PreviewResult, error) {
	if s == nil || s.DB == nil {
		return nil, ErrInvalidInput
	}
	return s.previewWithDB(ctx, s.DB, tenantID, scope, shopID, fileName, data, false)
}

func invalidDataRowCount(issues []ValidationIssue) int {
	lines := make(map[int]struct{})
	for _, issue := range issues {
		if issue.Line >= 2 {
			lines[issue.Line] = struct{}{}
		}
	}
	return len(lines)
}

func (s *Service) findImport(ctx context.Context, db *gorm.DB, tenantID int64, scope Scope, where string, values ...any) (*Import, error) {
	var row Import
	query := applyShopScope(db.WithContext(ctx).Where("tenant_id = ?", tenantID).Where(where, values...), scope, "shop_id")
	if err := query.Take(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) Confirm(ctx context.Context, tenantID int64, scope Scope, shopID uuid.UUID, fileName string, data []byte, expectedFileHash, expectedCalculationHash, idempotencyKey string, actor *uuid.UUID) (*ImportResult, error) {
	key, keyErr := normalizeKey(idempotencyKey)
	expectedFileHash = strings.ToLower(strings.TrimSpace(expectedFileHash))
	expectedCalculationHash = strings.ToLower(strings.TrimSpace(expectedCalculationHash))
	if s == nil || s.DB == nil || tenantID < 0 || shopID == uuid.Nil || keyErr != nil || len(expectedFileHash) != 64 || len(expectedCalculationHash) != 64 || fileHash(data) != expectedFileHash || !scopeAllowsShop(scope, shopID) {
		return nil, ErrInvalidInput
	}
	requestHash, err := hashValue(struct {
		ShopID          uuid.UUID `json:"shopId"`
		FileHash        string    `json:"fileHash"`
		CalculationHash string    `json:"calculationHash"`
	}{shopID, expectedFileHash, expectedCalculationHash})
	if err != nil {
		return nil, err
	}
	var result ImportResult
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, findErr := s.findImport(ctx, tx, tenantID, scope, "idempotency_key = ?", key); findErr == nil {
			if existing.ShopID != shopID || existing.RequestHash != requestHash {
				return ErrConflict
			}
			result = ImportResult{Import: *existing, Replayed: true}
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if existing, findErr := s.findImport(ctx, tx, tenantID, scope, "shop_id = ? AND file_hash = ?", shopID, expectedFileHash); findErr == nil {
			if existing.CalculationHash != expectedCalculationHash {
				return ErrConflict
			}
			result = ImportResult{Import: *existing, Replayed: true}
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
			TenantID: tenantID, ShopID: shopID, Platform: preview.Platform, ShopTimezone: preview.ShopTimezone,
			FileName: strings.TrimSpace(fileName), FileHash: expectedFileHash, PolicyVersion: PolicyVersion,
			CalculationHash: expectedCalculationHash, IdempotencyKey: key, RequestHash: requestHash,
			SourceRowCount: preview.SourceRows, ImportedSpends: preview.NewSpends, DuplicateSpends: preview.DuplicateSpends,
			AllocationCount: preview.AllocationCount, ImportedBy: actorPointer(actor), ConfirmedAt: now,
		}
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		ordersByLine := make(map[int][]PreviewOrder)
		for _, orderRow := range preview.Orders {
			if orderRow.Included {
				ordersByLine[orderRow.Line] = append(ordersByLine[orderRow.Line], orderRow)
			}
		}
		for _, row := range preview.Rows {
			if row.Disposition == DispositionDup {
				continue
			}
			spend := Spend{
				TenantID: tenantID, ImportID: batch.ID, ShopID: shopID, Platform: preview.Platform,
				ShopName: preview.ShopName, ShopTimezone: preview.ShopTimezone, SpendDate: row.SpendDate,
				Currency: row.Currency, AmountMinor: row.SpendMinor, SettlementCoverage: row.SettlementCoverage,
				PolicyVersion: PolicyVersion, CalculationHash: row.CalculationHash, RowHash: row.RowHash,
				EligibleOrderCount: row.EligibleOrderCount, ExcludedOrderCount: row.ExcludedOrderCount,
				AttributedBy: actorPointer(actor), AttributedAt: now,
			}
			if err := tx.Create(&spend).Error; err != nil {
				return err
			}
			allocations := make([]Allocation, 0, len(ordersByLine[row.Line]))
			for _, orderRow := range ordersByLine[row.Line] {
				if orderRow.AmountMinor == nil || orderRow.PaidAt == nil {
					return ErrConflict
				}
				allocations = append(allocations, Allocation{
					TenantID: tenantID, ImportID: batch.ID, SpendID: spend.ID, ShopID: shopID,
					OrderID: orderRow.OrderID, OrderNo: orderRow.OrderNo, PaidAt: orderRow.PaidAt.UTC(),
					AmountMinor: *orderRow.AmountMinor, Currency: row.Currency, PolicyVersion: PolicyVersion,
					CalculationHash: row.CalculationHash, AttributedBy: actorPointer(actor), AttributedAt: now,
				})
			}
			if len(allocations) > 0 {
				if err := tx.Create(&allocations).Error; err != nil {
					return err
				}
			}
		}
		result = ImportResult{Import: batch}
		return nil
	})
	if err == nil {
		return &result, nil
	}
	if isUniqueViolation(err) {
		if existing, findErr := s.findImport(ctx, s.DB, tenantID, scope, "idempotency_key = ? OR (shop_id = ? AND file_hash = ?)", key, shopID, expectedFileHash); findErr == nil && existing.ShopID == shopID && existing.CalculationHash == expectedCalculationHash {
			return &ImportResult{Import: *existing, Replayed: true}, nil
		}
		return nil, ErrConflict
	}
	return nil, err
}

func normalizeListQuery(input ListQuery) (ListQuery, error) {
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.SettlementCoverage = strings.ToLower(strings.TrimSpace(input.SettlementCoverage))
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 {
		input.PageSize = 20
	}
	if input.PageSize > MaxPageSize || (input.Currency != "" && !validCurrency(input.Currency)) || (input.SettlementCoverage != "" && !validCoverage(input.SettlementCoverage)) {
		return input, ErrInvalidInput
	}
	return input, nil
}

func (s *Service) spendsByID(ctx context.Context, tenantID int64, ids []uuid.UUID) (map[uuid.UUID]Spend, error) {
	result := make(map[uuid.UUID]Spend)
	if len(ids) == 0 {
		return result, nil
	}
	var rows []Spend
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func uniqueSpendIDs(rows []Allocation) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{})
	ids := make([]uuid.UUID, 0)
	for _, row := range rows {
		if _, ok := seen[row.SpendID]; !ok {
			seen[row.SpendID] = struct{}{}
			ids = append(ids, row.SpendID)
		}
	}
	return ids
}

func uniqueAllocationIDs(rows []Allocation) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func (s *Service) adjustmentsByAllocation(ctx context.Context, tenantID int64, ids []uuid.UUID) (map[uuid.UUID][]Adjustment, error) {
	result := make(map[uuid.UUID][]Adjustment)
	if len(ids) == 0 {
		return result, nil
	}
	var rows []Adjustment
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND allocation_id IN ?", tenantID, ids).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.AllocationID] = append(result[row.AllocationID], row)
	}
	return result, nil
}

func summarizeAllocation(allocation Allocation, spend Spend, adjustments []Adjustment) (AllocationSummary, error) {
	result := AllocationSummary{
		Allocation: allocation, SpendDate: spend.SpendDate, ShopName: spend.ShopName, Platform: spend.Platform,
		ShopTimezone: spend.ShopTimezone, SettlementCoverage: spend.SettlementCoverage, BaseSpendMinor: spend.AmountMinor,
		NetAmountMinor: allocation.AmountMinor,
	}
	if allocation.ID == uuid.Nil || allocation.OrderID == uuid.Nil || allocation.SpendID != spend.ID || allocation.ImportID != spend.ImportID || allocation.ShopID != spend.ShopID || allocation.Currency != spend.Currency || allocation.PolicyVersion != spend.PolicyVersion || allocation.CalculationHash != spend.CalculationHash || allocation.AmountMinor < 0 || allocation.AmountMinor > MaxJSONSafeInt64 || spend.AmountMinor <= 0 || spend.AmountMinor > MaxJSONSafeInt64 || spend.PolicyVersion != PolicyVersion || len(spend.CalculationHash) != 64 || !validCoverage(spend.SettlementCoverage) || !validCurrency(spend.Currency) {
		return result, ErrConflict
	}
	byID := make(map[uuid.UUID]Adjustment, len(adjustments))
	for _, row := range adjustments {
		byID[row.ID] = row
	}
	for _, row := range adjustments {
		if row.ID == uuid.Nil || row.AllocationID != allocation.ID || row.OrderID != allocation.OrderID || row.AmountMinor == 0 || row.AmountMinor < -MaxJSONSafeInt64 || row.AmountMinor > MaxJSONSafeInt64 || row.Currency != allocation.Currency {
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
	net, ok := safeAdd(allocation.AmountMinor, result.AdjustmentMinor)
	if !ok || net < 0 {
		return result, ErrConflict
	}
	result.NetAmountMinor = net
	switch spend.SettlementCoverage {
	case CoverageExcluded:
		result.ProfitStatus = "confirmed"
	case CoverageIncluded:
		result.ProfitStatus, result.ReasonCode, result.Reason = "mismatch", "advertising_in_platform_fee", "广告费用已包含在平台费用中，不得重复计入利润"
	default:
		result.ProfitStatus, result.ReasonCode, result.Reason = "blocked", "advertising_coverage_unknown", "广告费用的结算覆盖未知"
	}
	return result, nil
}

func (s *Service) ListAllocations(ctx context.Context, tenantID int64, scope Scope, raw ListQuery) (*AllocationList, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalidInput
	}
	input, err := normalizeListQuery(raw)
	if err != nil {
		return nil, err
	}
	query := s.DB.WithContext(ctx).Model(&Allocation{}).
		Joins("JOIN advertising_fee_spends AS spend ON spend.id = advertising_fee_allocations.spend_id AND spend.tenant_id = advertising_fee_allocations.tenant_id").
		Where("advertising_fee_allocations.tenant_id = ?", tenantID)
	query = applyShopScope(query, scope, "advertising_fee_allocations.shop_id")
	if input.OrderNo != "" {
		query = query.Where("LOWER(advertising_fee_allocations.order_no) LIKE ?", "%"+strings.ToLower(input.OrderNo)+"%")
	}
	if input.ShopID != nil {
		query = query.Where("advertising_fee_allocations.shop_id = ?", *input.ShopID)
	}
	if input.Currency != "" {
		query = query.Where("advertising_fee_allocations.currency = ?", input.Currency)
	}
	if input.SettlementCoverage != "" {
		query = query.Where("spend.settlement_coverage = ?", input.SettlementCoverage)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	rows := make([]Allocation, 0)
	if total > 0 {
		if err := query.Select("advertising_fee_allocations.*").Order("advertising_fee_allocations.attributed_at DESC, advertising_fee_allocations.id DESC").Offset((input.Page - 1) * input.PageSize).Limit(input.PageSize).Find(&rows).Error; err != nil {
			return nil, err
		}
	}
	spends, err := s.spendsByID(ctx, tenantID, uniqueSpendIDs(rows))
	if err != nil {
		return nil, err
	}
	adjustments, err := s.adjustmentsByAllocation(ctx, tenantID, uniqueAllocationIDs(rows))
	if err != nil {
		return nil, err
	}
	list := make([]AllocationSummary, 0, len(rows))
	for _, row := range rows {
		spend, ok := spends[row.SpendID]
		if !ok {
			return nil, ErrConflict
		}
		summary, summaryErr := summarizeAllocation(row, spend, adjustments[row.ID])
		if summaryErr != nil {
			return nil, summaryErr
		}
		list = append(list, summary)
	}
	return &AllocationList{List: list, Page: input.Page, PageSize: input.PageSize, Total: total, TotalPages: pagesOf(total, input.PageSize)}, nil
}

func (s *Service) GetAllocation(ctx context.Context, tenantID int64, scope Scope, id uuid.UUID) (*AllocationDetail, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var allocation Allocation
	query := applyShopScope(s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id), scope, "shop_id")
	if err := query.Take(&allocation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	spends, err := s.spendsByID(ctx, tenantID, []uuid.UUID{allocation.SpendID})
	if err != nil {
		return nil, err
	}
	adjustments, err := s.adjustmentsByAllocation(ctx, tenantID, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	spend, ok := spends[allocation.SpendID]
	if !ok {
		return nil, ErrConflict
	}
	summary, err := summarizeAllocation(allocation, spend, adjustments[id])
	if err != nil {
		return nil, err
	}
	return &AllocationDetail{AllocationSummary: summary, Adjustments: adjustments[id]}, nil
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
	query := applyShopScope(s.DB.WithContext(ctx).Model(&Import{}).Where("tenant_id = ?", tenantID), scope, "shop_id")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	rows := make([]Import, 0)
	if total > 0 {
		if err := query.Order("confirmed_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
			return nil, err
		}
	}
	return &ImportList{List: rows, Page: page, PageSize: pageSize, Total: total, TotalPages: pagesOf(total, pageSize)}, nil
}

func adjustmentRequestHash(allocationID uuid.UUID, factType string, amount int64, reason, key string, reverses *uuid.UUID) (string, error) {
	return hashValue(struct {
		AllocationID uuid.UUID  `json:"allocationId"`
		FactType     string     `json:"factType"`
		Amount       int64      `json:"amountMinor"`
		Reason       string     `json:"reason"`
		Key          string     `json:"idempotencyKey"`
		Reverses     *uuid.UUID `json:"reversesAdjustmentId,omitempty"`
	}{allocationID, factType, amount, reason, key, reverses})
}

func (s *Service) adjustmentReplay(ctx context.Context, tx *gorm.DB, tenantID int64, scope Scope, key, requestHash string) (*AdjustmentResult, error) {
	var existing Adjustment
	query := tx.WithContext(ctx).Table("advertising_fee_adjustments AS adjustment").
		Select("adjustment.*").
		Joins("JOIN advertising_fee_allocations AS allocation ON allocation.id = adjustment.allocation_id AND allocation.tenant_id = adjustment.tenant_id").
		Where("adjustment.tenant_id = ? AND adjustment.idempotency_key = ?", tenantID, key)
	query = applyShopScope(query, scope, "allocation.shop_id")
	if err := query.Take(&existing).Error; err != nil {
		return nil, err
	}
	if existing.RequestHash != requestHash {
		return nil, ErrConflict
	}
	return &AdjustmentResult{Adjustment: existing, Replayed: true}, nil
}

func (s *Service) lockedAllocation(ctx context.Context, tx *gorm.DB, tenantID int64, scope Scope, id uuid.UUID) (*Allocation, error) {
	var row Allocation
	query := applyShopScope(tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id), scope, "shop_id")
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) validateNetAmount(ctx context.Context, tx *gorm.DB, allocation Allocation, delta int64) error {
	var current int64
	if err := tx.WithContext(ctx).Model(&Adjustment{}).Where("tenant_id = ? AND allocation_id = ?", allocation.TenantID, allocation.ID).Select("COALESCE(SUM(amount_minor), 0)").Scan(&current).Error; err != nil {
		return err
	}
	net, ok := safeAdd(allocation.AmountMinor, current)
	if !ok {
		return ErrInvalidInput
	}
	net, ok = safeAdd(net, delta)
	if !ok || net < 0 {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) CreateAdjustment(ctx context.Context, tenantID int64, scope Scope, actor *uuid.UUID, allocationID uuid.UUID, input CreateAdjustmentInput) (*AdjustmentResult, error) {
	reason := strings.TrimSpace(input.Reason)
	key, err := normalizeKey(input.IdempotencyKey)
	if s == nil || s.DB == nil || allocationID == uuid.Nil || input.AmountMinor == 0 || input.AmountMinor < -MaxJSONSafeInt64 || input.AmountMinor > MaxJSONSafeInt64 || len([]rune(reason)) < 2 || len([]rune(reason)) > 500 || err != nil {
		return nil, ErrInvalidInput
	}
	requestHash, err := adjustmentRequestHash(allocationID, FactAdjustment, input.AmountMinor, reason, key, nil)
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
		allocation, findErr := s.lockedAllocation(ctx, tx, tenantID, scope, allocationID)
		if findErr != nil {
			return findErr
		}
		if err := s.validateNetAmount(ctx, tx, *allocation, input.AmountMinor); err != nil {
			return err
		}
		row := Adjustment{TenantID: tenantID, AllocationID: allocation.ID, OrderID: allocation.OrderID, FactType: FactAdjustment, AmountMinor: input.AmountMinor, Currency: allocation.Currency, Reason: reason, IdempotencyKey: key, RequestHash: requestHash, ActorID: actorPointer(actor)}
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
		if replay, findErr := s.adjustmentReplay(ctx, s.DB, tenantID, scope, key, requestHash); findErr == nil {
			return replay, nil
		}
		return nil, ErrConflict
	}
	return nil, err
}

func (s *Service) ReverseAdjustment(ctx context.Context, tenantID int64, scope Scope, actor *uuid.UUID, allocationID, adjustmentID uuid.UUID, input ReverseAdjustmentInput) (*AdjustmentResult, error) {
	reason := strings.TrimSpace(input.Reason)
	key, err := normalizeKey(input.IdempotencyKey)
	if s == nil || s.DB == nil || allocationID == uuid.Nil || adjustmentID == uuid.Nil || len([]rune(reason)) < 2 || len([]rune(reason)) > 500 || err != nil {
		return nil, ErrInvalidInput
	}
	var result *AdjustmentResult
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		allocation, findErr := s.lockedAllocation(ctx, tx, tenantID, scope, allocationID)
		if findErr != nil {
			return findErr
		}
		var target Adjustment
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND allocation_id = ? AND id = ?", tenantID, allocationID, adjustmentID).Take(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if target.FactType != FactAdjustment || target.ReversesAdjustmentID != nil || target.AmountMinor == math.MinInt64 {
			return ErrConflict
		}
		amount := -target.AmountMinor
		requestHash, hashErr := adjustmentRequestHash(allocationID, FactReversal, amount, reason, key, &target.ID)
		if hashErr != nil {
			return hashErr
		}
		if replay, replayErr := s.adjustmentReplay(ctx, tx, tenantID, scope, key, requestHash); replayErr == nil {
			result = replay
			return nil
		} else if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}
		var reversalCount int64
		if err := tx.WithContext(ctx).Model(&Adjustment{}).Where("tenant_id = ? AND reverses_adjustment_id = ?", tenantID, target.ID).Count(&reversalCount).Error; err != nil {
			return err
		}
		if reversalCount != 0 {
			return ErrConflict
		}
		if err := s.validateNetAmount(ctx, tx, *allocation, amount); err != nil {
			return err
		}
		row := Adjustment{TenantID: tenantID, AllocationID: allocation.ID, OrderID: allocation.OrderID, FactType: FactReversal, AmountMinor: amount, Currency: allocation.Currency, Reason: reason, ReversesAdjustmentID: &target.ID, IdempotencyKey: key, RequestHash: requestHash, ActorID: actorPointer(actor)}
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

func pagesOf(total int64, pageSize int) int {
	if total == 0 || pageSize < 1 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}

func (s *Service) ProfitabilityFeesForOrders(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID]ProfitabilityFact, error) {
	result := make(map[uuid.UUID]ProfitabilityFact)
	if s == nil || s.DB == nil || len(orderIDs) == 0 {
		return result, nil
	}
	var requested []Allocation
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND order_id IN ?", tenantID, orderIDs).Find(&requested).Error; err != nil {
		return nil, err
	}
	if len(requested) == 0 {
		return result, nil
	}
	spendIDs := uniqueSpendIDs(requested)
	spends, err := s.spendsByID(ctx, tenantID, spendIDs)
	if err != nil {
		return nil, err
	}
	var allAllocations []Allocation
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND spend_id IN ?", tenantID, spendIDs).Order("paid_at ASC, order_no ASC, order_id ASC").Find(&allAllocations).Error; err != nil {
		return nil, err
	}
	allBySpend := make(map[uuid.UUID][]Allocation)
	for _, row := range allAllocations {
		allBySpend[row.SpendID] = append(allBySpend[row.SpendID], row)
	}
	adjustments, err := s.adjustmentsByAllocation(ctx, tenantID, uniqueAllocationIDs(requested))
	if err != nil {
		return nil, err
	}
	for _, allocation := range requested {
		spend, exists := spends[allocation.SpendID]
		fact := ProfitabilityFact{OrderID: allocation.OrderID, ImportID: allocation.ImportID, SpendID: allocation.SpendID, AllocationID: allocation.ID, Currency: allocation.Currency, SourceAt: allocation.AttributedAt}
		if !exists {
			fact.Status, fact.ReasonCode, fact.Reason = "blocked", "advertising_spend_missing", "广告费用来源事实缺失"
			result[allocation.OrderID] = fact
			continue
		}
		group := allBySpend[spend.ID]
		var total int64
		groupValid := len(group) == spend.EligibleOrderCount && spend.EligibleOrderCount > 0
		seenOrders := make(map[uuid.UUID]struct{}, len(group))
		for _, item := range group {
			if item.SpendID != spend.ID || item.ImportID != spend.ImportID || item.ShopID != spend.ShopID || item.Currency != spend.Currency || item.PolicyVersion != spend.PolicyVersion || item.CalculationHash != spend.CalculationHash || item.AmountMinor < 0 || item.AmountMinor > MaxJSONSafeInt64 {
				groupValid = false
			}
			if _, duplicate := seenOrders[item.OrderID]; duplicate {
				groupValid = false
			}
			seenOrders[item.OrderID] = struct{}{}
			var ok bool
			total, ok = safeAdd(total, item.AmountMinor)
			groupValid = groupValid && ok
		}
		if total != spend.AmountMinor {
			groupValid = false
		}
		if !groupValid {
			fact.Status, fact.ReasonCode, fact.Reason = "blocked", "advertising_allocation_invalid", "广告费用分配事实不完整或金额不守恒"
			result[allocation.OrderID] = fact
			continue
		}
		summary, summaryErr := summarizeAllocation(allocation, spend, adjustments[allocation.ID])
		if summaryErr != nil {
			fact.Status, fact.ReasonCode, fact.Reason = "blocked", "advertising_facts_invalid", "广告费用调整事实无效"
			result[allocation.OrderID] = fact
			continue
		}
		fact.AmountMinor, fact.Status = summary.NetAmountMinor, summary.ProfitStatus
		fact.ReasonCode, fact.Reason = summary.ReasonCode, summary.Reason
		for _, adjustment := range adjustments[allocation.ID] {
			fact.AdjustmentIDs = append(fact.AdjustmentIDs, adjustment.ID)
		}
		result[allocation.OrderID] = fact
	}
	return result, nil
}

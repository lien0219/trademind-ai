package settlement

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvalidInput = errors.New("invalid settlement input")
	ErrConflict     = errors.New("settlement conflict")
	ErrTooManyRows  = errors.New("settlement result exceeds 5000 rows; narrow the filters")
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

func (s *Service) Preview(ctx context.Context, tenantID int64, scope Scope, shopID uuid.UUID, fileName string, data []byte) (*PreviewResult, error) {
	if s == nil || s.DB == nil || tenantID < 0 || shopID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	repo := repository{db: s.DB}
	shop, err := repo.shop(ctx, tenantID, shopID)
	if err != nil {
		return nil, err
	}
	if !scopeAllowsShop(scope, shopID) {
		return nil, gorm.ErrRecordNotFound
	}
	rows, issues := parseCSV(data)
	result := &PreviewResult{
		FileName: fileName, FileHash: fileHash(data), ShopID: shopID, ShopName: shop.Name,
		Platform: shop.Platform, SourceRows: len(rows) + invalidDataRowCount(issues), Rows: rows, Issues: issues,
	}
	if len(rows) > 0 {
		externalIDs := make([]string, 0, len(rows))
		for _, row := range rows {
			externalIDs = append(externalIDs, row.ExternalTransactionID)
		}
		existing, findErr := repo.existingTransactions(ctx, tenantID, shopID, shop.Platform, externalIDs, false)
		if findErr != nil {
			return nil, findErr
		}
		result.markExisting(existing)
	}
	result.Valid = len(result.Issues) == 0
	return result, nil
}

func (s *Service) Confirm(ctx context.Context, tenantID int64, scope Scope, shopID uuid.UUID, fileName string, data []byte, expectedHash, idempotencyKey string, importedBy *uuid.UUID) (*ImportResult, error) {
	expectedHash = strings.ToLower(strings.TrimSpace(expectedHash))
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(expectedHash) != 64 || len(idempotencyKey) < 8 || len(idempotencyKey) > 128 {
		return nil, ErrInvalidInput
	}
	preview, err := s.Preview(ctx, tenantID, scope, shopID, fileName, data)
	if err != nil {
		return nil, err
	}
	if preview.FileHash != expectedHash || !preview.Valid {
		return nil, ErrConflict
	}

	var result ImportResult
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := repository{db: tx}
		if existing, findErr := repo.findImportByIdempotency(ctx, tenantID, idempotencyKey); findErr == nil {
			if existing.ShopID != shopID || existing.Platform != preview.Platform || existing.FileHash != expectedHash {
				return ErrConflict
			}
			result = ImportResult{Import: *existing, Replayed: true}
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if existing, findErr := repo.findImportByFile(ctx, tenantID, shopID, expectedHash); findErr == nil {
			if existing.Platform != preview.Platform {
				return ErrConflict
			}
			result = ImportResult{Import: *existing, Replayed: true}
			return nil
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}

		externalIDs := make([]string, 0, len(preview.Rows))
		for _, row := range preview.Rows {
			externalIDs = append(externalIDs, row.ExternalTransactionID)
		}
		existing, findErr := repo.existingTransactions(ctx, tenantID, shopID, preview.Platform, externalIDs, true)
		if findErr != nil {
			return findErr
		}
		existingByID := make(map[string]Transaction, len(existing))
		for _, row := range existing {
			existingByID[row.ExternalTransactionID] = row
		}
		newRows := make([]Transaction, 0, len(preview.Rows))
		for _, row := range preview.Rows {
			if found, ok := existingByID[row.ExternalTransactionID]; ok {
				if found.RowHash != row.RowHash {
					return ErrConflict
				}
				continue
			}
			newRows = append(newRows, transactionFromCSV(tenantID, shopID, preview.Platform, row, importedBy))
		}
		batch := Import{
			TenantID: tenantID, ShopID: shopID, Platform: preview.Platform, FileName: strings.TrimSpace(fileName),
			FileHash: expectedHash, IdempotencyKey: idempotencyKey, SourceRowCount: len(preview.Rows),
			ImportedRows: len(newRows), DuplicateRows: len(preview.Rows) - len(newRows), ImportedBy: importedBy,
			ConfirmedAt: s.now(),
		}
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		for index := range newRows {
			newRows[index].ImportID = batch.ID
		}
		if len(newRows) > 0 {
			if err := tx.Create(&newRows).Error; err != nil {
				return err
			}
		}
		result = ImportResult{Import: batch}
		return nil
	})
	if err == nil {
		return &result, nil
	}
	// A concurrent confirmation may win either unique index. Resolve an exact
	// replay after rollback; divergent data remains a conflict.
	repo := repository{db: s.DB}
	if existing, findErr := repo.findImportByIdempotency(ctx, tenantID, idempotencyKey); findErr == nil && existing.ShopID == shopID && existing.FileHash == expectedHash {
		if existing.Platform == preview.Platform {
			return &ImportResult{Import: *existing, Replayed: true}, nil
		}
	}
	if existing, findErr := repo.findImportByFile(ctx, tenantID, shopID, expectedHash); findErr == nil {
		if existing.Platform == preview.Platform {
			return &ImportResult{Import: *existing, Replayed: true}, nil
		}
	}
	if errors.Is(err, ErrConflict) || isUniqueConstraintError(err) {
		return nil, ErrConflict
	}
	return nil, fmt.Errorf("confirm settlement import: %w", err)
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "unique violation") ||
		strings.Contains(message, "constraint failed") ||
		strings.Contains(message, "sqlstate 23505") ||
		strings.Contains(message, "error 1062")
}

func (p *PreviewResult) markExisting(existing []Transaction) {
	byID := make(map[string]Transaction, len(existing))
	for _, row := range existing {
		byID[row.ExternalTransactionID] = row
	}
	for index := range p.Rows {
		found, ok := byID[p.Rows[index].ExternalTransactionID]
		if !ok {
			p.NewRows++
			continue
		}
		if found.RowHash != p.Rows[index].RowHash {
			p.Issues = append(p.Issues, ValidationIssue{Line: p.Rows[index].Line, Field: CSVHeaders[0], Code: "external_transaction_conflict", Message: "外部交易号已存在，但账单内容不同"})
			continue
		}
		p.Rows[index].Disposition = DispositionDuplicate
		p.DuplicateRows++
	}
}

func transactionFromCSV(tenantID int64, shopID uuid.UUID, platform string, row CSVRow, importedBy *uuid.UUID) Transaction {
	groupID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("settlement:%d:%s:%s", tenantID, shopID.String(), row.OrderNo)))
	return Transaction{
		TenantID: tenantID, ReconciliationID: groupID, ShopID: shopID, Platform: platform,
		ExternalTransactionID: row.ExternalTransactionID, OrderNo: row.OrderNo, Currency: row.Currency,
		OrderGrossMinor: row.OrderGrossMinor, PlatformFeeMinor: row.PlatformFeeMinor,
		SettlementAmountMinor: row.SettlementAmountMinor, SettledAt: row.SettledAt,
		RowHash: row.RowHash, ImportedBy: importedBy,
	}
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

func normalizeListQuery(q ListQuery) (ListQuery, error) {
	q.OrderNo = strings.TrimSpace(q.OrderNo)
	q.Platform = strings.ToLower(strings.TrimSpace(q.Platform))
	q.Currency = strings.ToUpper(strings.TrimSpace(q.Currency))
	q.Status = strings.ToLower(strings.TrimSpace(q.Status))
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > MaxPageSize || (q.Currency != "" && !validCurrency(q.Currency)) || (q.Start != nil && q.End != nil && q.Start.After(*q.End)) {
		return q, ErrInvalidInput
	}
	if q.Status != "" && q.Status != StatusMatched && q.Status != StatusPending && q.Status != StatusMismatch && q.Status != StatusBlocked {
		return q, ErrInvalidInput
	}
	return q, nil
}

func (s *Service) List(ctx context.Context, tenantID int64, scope Scope, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalidInput
	}
	q, err := normalizeListQuery(q)
	if err != nil {
		return nil, err
	}
	derived := q.Export || q.Status != ""
	offset, limit := (q.Page-1)*q.PageSize, q.PageSize
	if derived {
		offset, limit = 0, MaxExportRows+1
	}
	repo := repository{db: s.DB}
	groups, rawTotal, err := repo.listGroups(ctx, tenantID, scope, q, offset, limit)
	if err != nil {
		return nil, err
	}
	if derived && rawTotal > MaxExportRows {
		return nil, ErrTooManyRows
	}
	rows, err := s.calculateGroups(ctx, tenantID, groups)
	if err != nil {
		return nil, err
	}
	total := rawTotal
	if q.Status != "" {
		filtered := make([]ReconciliationRow, 0, len(rows))
		for _, row := range rows {
			if row.Status == q.Status {
				filtered = append(filtered, row)
			}
		}
		total = int64(len(filtered))
		if !q.Export {
			start := (q.Page - 1) * q.PageSize
			if start > len(filtered) {
				start = len(filtered)
			}
			end := start + q.PageSize
			if end > len(filtered) {
				end = len(filtered)
			}
			filtered = filtered[start:end]
		}
		rows = filtered
	}
	if q.Export {
		q.Page, q.PageSize = 1, MaxExportRows
	}
	return &ListResult{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: pagesOf(total, q.PageSize)}, nil
}

func (s *Service) Get(ctx context.Context, tenantID int64, scope Scope, id uuid.UUID) (*ReconciliationDetail, error) {
	if s == nil || s.DB == nil || id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	repo := repository{db: s.DB}
	group, err := repo.getGroup(ctx, tenantID, scope, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.calculateGroups(ctx, tenantID, []groupFact{*group})
	if err != nil {
		return nil, err
	}
	facts, err := repo.listTransactions(ctx, tenantID, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &ReconciliationDetail{ReconciliationRow: rows[0], Transactions: facts}, nil
}

func (s *Service) calculateGroups(ctx context.Context, tenantID int64, groups []groupFact) ([]ReconciliationRow, error) {
	if len(groups) == 0 {
		return []ReconciliationRow{}, nil
	}
	ids := make([]uuid.UUID, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	repo := repository{db: s.DB}
	facts, err := repo.listTransactions(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	orders, err := repo.listOrders(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	factsByGroup := make(map[uuid.UUID][]Transaction)
	ordersByGroup := make(map[uuid.UUID][]orderFact)
	for _, fact := range facts {
		factsByGroup[fact.ReconciliationID] = append(factsByGroup[fact.ReconciliationID], fact)
	}
	for _, order := range orders {
		ordersByGroup[order.ReconciliationID] = append(ordersByGroup[order.ReconciliationID], order)
	}
	rows := make([]ReconciliationRow, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, calculateGroup(group, factsByGroup[group.ID], ordersByGroup[group.ID]))
	}
	return rows, nil
}

func calculateGroup(group groupFact, facts []Transaction, orders []orderFact) ReconciliationRow {
	row := ReconciliationRow{
		ID: group.ID, ShopID: group.ShopID, ShopName: group.ShopName, Platform: group.Platform,
		OrderNo: group.OrderNo, TransactionCount: len(facts), Status: StatusMatched, Issues: []Issue{},
	}
	if group.ShopName == "" {
		promoteStatus(&row, StatusBlocked)
		row.Issues = append(row.Issues, Issue{Code: "shop_missing", Message: "账单店铺已不存在或不可用"})
	}
	if len(facts) == 0 {
		promoteStatus(&row, StatusBlocked)
		row.Issues = append(row.Issues, Issue{Code: "settlement_facts_missing", Message: "结算交易事实缺失"})
		return row
	}
	currencies := make(map[string]struct{})
	var gross, fee, settled int64
	safe := true
	groupShapeValid := true
	for _, fact := range facts {
		if fact.ShopID != group.ShopID || fact.OrderNo != group.OrderNo || !strings.EqualFold(fact.Platform, group.Platform) {
			groupShapeValid = false
		}
		if row.FirstSettledAt.IsZero() || fact.SettledAt.Before(row.FirstSettledAt) {
			row.FirstSettledAt = fact.SettledAt
		}
		if fact.SettledAt.After(row.LastSettledAt) {
			row.LastSettledAt = fact.SettledAt
		}
		if fact.CreatedAt.After(row.LastImportedAt) {
			row.LastImportedAt = fact.CreatedAt
		}
		currencies[fact.Currency] = struct{}{}
		var ok bool
		gross, ok = safeAdd(gross, fact.OrderGrossMinor)
		safe = safe && ok && jsonSafeInteger(gross)
		fee, ok = safeAdd(fee, fact.PlatformFeeMinor)
		safe = safe && ok && jsonSafeInteger(fee)
		settled, ok = safeAdd(settled, fact.SettlementAmountMinor)
		safe = safe && ok && jsonSafeInteger(settled)
		if expected, relationOK := safeSubtract(fact.OrderGrossMinor, fact.PlatformFeeMinor); !relationOK || expected != fact.SettlementAmountMinor {
			safe = false
		}
	}
	if !groupShapeValid {
		promoteStatus(&row, StatusBlocked)
		row.Issues = append(row.Issues, Issue{Code: "settlement_group_conflict", Message: "同一对账组存在店铺、平台或订单号冲突"})
	}
	if len(currencies) == 1 {
		for currency := range currencies {
			row.Currency = currency
		}
	} else {
		promoteStatus(&row, StatusBlocked)
		row.Issues = append(row.Issues, Issue{Code: "settlement_currency_conflict", Message: "同一订单存在多个账单币种"})
	}
	if safe {
		row.OrderGrossMinor, row.PlatformFeeMinor, row.SettlementAmountMinor = int64Ptr(gross), int64Ptr(fee), int64Ptr(settled)
		if expected, ok := safeSubtract(gross, fee); !ok || expected != settled {
			promoteStatus(&row, StatusMismatch)
			row.Issues = append(row.Issues, Issue{Code: "settlement_total_mismatch", Message: "聚合结算金额不等于交易总额减平台费用"})
		}
	} else {
		promoteStatus(&row, StatusBlocked)
		row.Issues = append(row.Issues, Issue{Code: "settlement_amount_unsafe", Message: "结算金额聚合超出安全范围或事实关系无效"})
	}
	if len(orders) == 0 {
		if row.Status == StatusMatched {
			row.Status = StatusPending
		}
		row.Issues = append(row.Issues, Issue{Code: "local_order_missing", Message: "尚未找到同店铺本地订单"})
		return row
	}
	if len(orders) > 1 {
		promoteStatus(&row, StatusBlocked)
		row.Issues = append(row.Issues, Issue{Code: "local_order_ambiguous", Message: "同一账单匹配到多个本地订单"})
		return row
	}
	row.OrderID = &orders[0].ID
	orderAmount, err := parseMajorToMinor(orders[0].TotalAmountText, orders[0].Currency)
	if err != nil || orderAmount < 0 || !jsonSafeInteger(orderAmount) {
		promoteStatus(&row, StatusBlocked)
		row.Issues = append(row.Issues, Issue{Code: "order_amount_invalid", Message: "本地订单金额无法精确转换为最小货币单位"})
		return row
	}
	row.OrderAmountMinor = &orderAmount
	if row.Currency != "" && !strings.EqualFold(row.Currency, orders[0].Currency) {
		promoteStatus(&row, StatusMismatch)
		row.Issues = append(row.Issues, Issue{Code: "order_currency_mismatch", Message: "账单币种与本地订单币种不一致"})
	}
	if !strings.EqualFold(row.Platform, orders[0].Platform) {
		promoteStatus(&row, StatusMismatch)
		row.Issues = append(row.Issues, Issue{Code: "order_platform_mismatch", Message: "账单平台与本地订单平台不一致"})
	}
	if row.OrderGrossMinor != nil && *row.OrderGrossMinor != orderAmount {
		promoteStatus(&row, StatusMismatch)
		row.Issues = append(row.Issues, Issue{Code: "order_amount_mismatch", Message: "账单交易总额与本地订单金额不一致"})
	}
	return row
}

func promoteStatus(row *ReconciliationRow, candidate string) {
	if reconciliationStatusPriority(candidate) > reconciliationStatusPriority(row.Status) {
		row.Status = candidate
	}
}

func reconciliationStatusPriority(status string) int {
	switch status {
	case StatusBlocked:
		return 3
	case StatusMismatch:
		return 2
	case StatusPending:
		return 1
	default:
		return 0
	}
}

func (s *Service) PlatformFeesForOrders(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID]PlatformFeeFact, error) {
	result := make(map[uuid.UUID]PlatformFeeFact)
	if s == nil || s.DB == nil || len(orderIDs) == 0 {
		return result, nil
	}
	repo := repository{db: s.DB}
	groupIDs, err := repo.groupIDsForOrders(ctx, tenantID, orderIDs)
	if err != nil || len(groupIDs) == 0 {
		return result, err
	}
	groups, err := repo.groupsByIDs(ctx, tenantID, groupIDs)
	if err != nil {
		return nil, err
	}
	rows, err := s.calculateGroups(ctx, tenantID, groups)
	if err != nil {
		return nil, err
	}
	facts, err := repo.listTransactions(ctx, tenantID, groupIDs)
	if err != nil {
		return nil, err
	}
	factsByGroup := make(map[uuid.UUID][]Transaction)
	for _, fact := range facts {
		factsByGroup[fact.ReconciliationID] = append(factsByGroup[fact.ReconciliationID], fact)
	}
	for _, row := range rows {
		if row.OrderID == nil {
			continue
		}
		fact := PlatformFeeFact{OrderID: *row.OrderID, ReconciliationID: row.ID, Currency: row.Currency, Status: row.Status, SourceAt: row.LastSettledAt}
		for _, transaction := range factsByGroup[row.ID] {
			fact.TransactionIDs = append(fact.TransactionIDs, transaction.ID)
		}
		if row.Status == StatusMatched && row.PlatformFeeMinor != nil {
			fact.AmountMinor = *row.PlatformFeeMinor
		} else if len(row.Issues) > 0 {
			fact.ReasonCode, fact.Reason = row.Issues[0].Code, row.Issues[0].Message
		}
		result[*row.OrderID] = fact
	}
	return result, nil
}

func parseMajorToMinor(raw, currency string) (int64, error) {
	if !validCurrency(strings.ToUpper(strings.TrimSpace(currency))) {
		return 0, fmt.Errorf("unsupported currency")
	}
	value, ok := new(big.Rat).SetString(strings.TrimSpace(raw))
	if !ok {
		return 0, fmt.Errorf("invalid decimal amount")
	}
	scale := int64(1)
	for index := 0; index < currencyExponent(currency); index++ {
		scale *= 10
	}
	value.Mul(value, big.NewRat(scale, 1))
	if value.Denom().Cmp(big.NewInt(1)) != 0 || !value.Num().IsInt64() {
		return 0, fmt.Errorf("amount precision exceeds currency exponent")
	}
	return value.Num().Int64(), nil
}

func currencyExponent(currency string) int {
	switch strings.ToUpper(currency) {
	case "BIF", "CLP", "DJF", "GNF", "JPY", "KMF", "KRW", "MGA", "PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF":
		return 0
	case "BHD", "IQD", "JOD", "KWD", "LYD", "OMR", "TND":
		return 3
	case "CLF", "UYW":
		return 4
	default:
		return 2
	}
}

func int64Ptr(value int64) *int64 { return &value }

func pagesOf(total int64, pageSize int) int {
	if total == 0 || pageSize < 1 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}

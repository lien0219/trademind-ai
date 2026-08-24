package salesreturn

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func refundExecutionQuery(db *gorm.DB, tenantID int64) *gorm.DB {
	return db.Model(&RefundExecution{}).
		Select("refund_executions.*, sales_returns.return_no, orders.order_no").
		Joins("JOIN sales_returns ON sales_returns.id = refund_executions.sales_return_id AND sales_returns.tenant_id = refund_executions.tenant_id AND sales_returns.deleted_at IS NULL").
		Joins("JOIN orders ON orders.id = refund_executions.order_id AND orders.tenant_id = refund_executions.tenant_id AND orders.deleted_at IS NULL").
		Where("refund_executions.tenant_id = ?", tenantID)
}

func enrichRefundPlatformReviews(ctx context.Context, db *gorm.DB, tenantID int64, rows []RefundExecution) error {
	if len(rows) == 0 {
		return nil
	}
	explicitIDs := make([]uuid.UUID, 0, len(rows))
	returnIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		returnIDs = append(returnIDs, rows[i].SalesReturnID)
		if rows[i].PlatformAfterSaleID != nil && *rows[i].PlatformAfterSaleID != uuid.Nil {
			explicitIDs = append(explicitIDs, *rows[i].PlatformAfterSaleID)
		}
	}
	query := db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if len(explicitIDs) > 0 {
		query = query.Where("id IN ? OR sales_return_id IN ?", explicitIDs, returnIDs)
	} else {
		query = query.Where("sales_return_id IN ?", returnIDs)
	}
	var facts []PlatformAfterSale
	if err := query.Order("updated_at DESC, id DESC").Find(&facts).Error; err != nil {
		return fmt.Errorf("load refund platform reviews: %w", err)
	}
	byID := make(map[uuid.UUID]*PlatformAfterSale, len(facts))
	byReturn := make(map[uuid.UUID]*PlatformAfterSale, len(facts))
	returnFactCounts := make(map[uuid.UUID]int, len(facts))
	for i := range facts {
		fact := &facts[i]
		byID[fact.ID] = fact
		if fact.SalesReturnID != nil {
			returnFactCounts[*fact.SalesReturnID]++
			if _, exists := byReturn[*fact.SalesReturnID]; !exists {
				byReturn[*fact.SalesReturnID] = fact
			}
		}
	}
	for i := range rows {
		var fact *PlatformAfterSale
		if rows[i].PlatformAfterSaleID != nil {
			fact = byID[*rows[i].PlatformAfterSaleID]
		} else {
			if returnFactCounts[rows[i].SalesReturnID] > 1 {
				rows[i].PlatformReview = &RefundPlatformReview{Status: PlatformReconciliationBlocked, Reason: "multiple_platform_refund_facts"}
				continue
			}
			fact = byReturn[rows[i].SalesReturnID]
		}
		rows[i].PlatformReview = buildRefundPlatformReview(&rows[i], fact)
	}
	return nil
}

func buildRefundPlatformReview(row *RefundExecution, fact *PlatformAfterSale) *RefundPlatformReview {
	review := &RefundPlatformReview{Status: PlatformReconciliationPending, Reason: "platform_fact_missing"}
	if fact == nil {
		if row.PlatformAfterSaleID != nil {
			review.Status, review.Reason = PlatformReconciliationBlocked, "linked_platform_fact_missing"
		}
		return review
	}
	factID := fact.ID
	review.PlatformAfterSaleID = &factID
	review.Platform = fact.Platform
	review.PlatformStatus = fact.PlatformStatus
	review.ExternalAfterSaleID = fact.ExternalAfterSaleID
	review.RefundAmountMinor = fact.RefundAmountMinor
	review.Currency = fact.Currency
	review.PlatformUpdatedAt = fact.PlatformUpdatedAt
	if fact.SalesReturnID == nil || *fact.SalesReturnID != row.SalesReturnID || fact.OrderID == nil || *fact.OrderID != row.OrderID || fact.InternalShopID == nil || row.InternalShopID == nil || *fact.InternalShopID != *row.InternalShopID || fact.ReconciliationStatus == PlatformReconciliationBlocked {
		review.Status, review.Reason = PlatformReconciliationBlocked, "platform_fact_link_blocked"
		return review
	}
	if fact.ReconciliationStatus == PlatformReconciliationMismatch {
		review.Status, review.Reason = PlatformReconciliationMismatch, "platform_fact_reconciliation_mismatch"
		return review
	}
	if fact.ReconciliationStatus != PlatformReconciliationMatched {
		review.Status, review.Reason = PlatformReconciliationPending, "platform_fact_reconciliation_pending"
		return review
	}
	if fact.RefundAmountMinor != row.RefundAmountMinor || !strings.EqualFold(strings.TrimSpace(fact.Currency), strings.TrimSpace(row.Currency)) {
		review.Status, review.Reason = PlatformReconciliationMismatch, "platform_refund_amount_or_currency_mismatch"
		return review
	}
	platformResult := platformRefundResult(fact.PlatformStatus)
	if row.Status == RefundExecutionStatusPending {
		review.Status, review.Reason = PlatformReconciliationPending, "execution_result_pending"
		return review
	}
	if platformResult == "" {
		review.Status, review.Reason = PlatformReconciliationPending, "platform_status_pending"
		return review
	}
	if row.Status == platformResult || (row.Status == RefundExecutionStatusCancelled && platformResult == RefundExecutionStatusFailed) {
		review.Status, review.Reason = PlatformReconciliationMatched, "refund_result_matched"
		return review
	}
	review.Status, review.Reason = PlatformReconciliationMismatch, "refund_result_mismatch"
	return review
}

func requireMatchingPlatformFact(tx *gorm.DB, tenantID int64, salesReturn SalesReturn, id uuid.UUID, requireFinal bool) (*PlatformAfterSale, error) {
	var fact PlatformAfterSale
	if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&fact).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRefundPlatformFactConflict
	} else if err != nil {
		return nil, err
	}
	if fact.SalesReturnID == nil || *fact.SalesReturnID != salesReturn.ID || fact.OrderID == nil || *fact.OrderID != salesReturn.OrderID || fact.ReconciliationStatus != PlatformReconciliationMatched || fact.RefundAmountMinor != salesReturn.RefundAmountMinor || !strings.EqualFold(strings.TrimSpace(fact.Currency), strings.TrimSpace(salesReturn.Currency)) {
		return nil, ErrRefundPlatformFactConflict
	}
	if requireFinal && platformRefundResult(fact.PlatformStatus) == "" {
		return nil, ErrRefundPlatformFactPending
	}
	return &fact, nil
}

func platformRefundResult(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "refunded", "refund_success", "completed", "complete":
		return RefundExecutionStatusSucceeded
	case "failed", "refund_failed", "rejected", "cancelled", "canceled", "closed":
		return RefundExecutionStatusFailed
	default:
		return ""
	}
}

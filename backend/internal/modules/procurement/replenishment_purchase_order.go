package procurement

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// CreatePurchaseOrderFromReplenishment validates the user's selected
// suggestions against a fresh, tenant-scoped read before creating a local
// draft through the existing purchase-order lifecycle. It deliberately does
// not call a supplier, platform, queue, or worker.
func (s *Service) CreatePurchaseOrderFromReplenishment(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateReplenishmentPurchaseOrderInput) (*PurchaseOrder, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	remark := strings.TrimSpace(in.Remark)
	if tenantID < 0 || in.WarehouseID == uuid.Nil || in.SupplierID == uuid.Nil || len(key) < 8 || len(key) > 128 || len(in.Items) == 0 || len(in.Items) > maxOrderItems || len([]rune(remark)) > 1000 {
		return nil, ErrInvalidInput
	}
	seen := make(map[uuid.UUID]struct{}, len(in.Items))
	skuIDs := make([]uuid.UUID, 0, len(in.Items))
	for _, item := range in.Items {
		if item.ProductSKUID == uuid.Nil || item.Quantity < 1 || item.Quantity > maxQuantity || len(strings.TrimSpace(item.SuggestionHash)) != 64 {
			return nil, ErrInvalidInput
		}
		if _, ok := seen[item.ProductSKUID]; ok {
			return nil, ErrInvalidInput
		}
		seen[item.ProductSKUID] = struct{}{}
		skuIDs = append(skuIDs, item.ProductSKUID)
	}

	result, err := s.ListReplenishmentSuggestions(ctx, tenantID, ReplenishmentQuery{
		WarehouseID:   in.WarehouseID,
		ProductSKUIDs: skuIDs,
		Page:          1,
		PageSize:      maxOrderItems,
	})
	if err != nil {
		return nil, err
	}
	bySKU := make(map[uuid.UUID]ReplenishmentSuggestion, len(result.List))
	for _, row := range result.List {
		bySKU[row.ProductSKUID] = row
	}

	items := make([]CreatePurchaseOrderItemInput, 0, len(in.Items))
	currency := ""
	for _, item := range in.Items {
		row, ok := bySKU[item.ProductSKUID]
		if !ok {
			return nil, ErrReplenishmentStale
		}
		if !strings.EqualFold(strings.TrimSpace(item.SuggestionHash), row.SuggestionHash) {
			return nil, ErrReplenishmentStale
		}
		if row.Status != "actionable" && row.Status != "blocked_supplier_selection" {
			return nil, fmt.Errorf("%w: %s", ErrReplenishmentBlocked, row.Status)
		}

		option, ok := replenishmentSupplierOptionFor(row.SupplierOptions, in.SupplierID, item.SupplierSKUID)
		if !ok {
			return nil, ErrReplenishmentSupplierInvalid
		}
		minOrderQty := option.MinOrderQty
		if minOrderQty < 1 {
			minOrderQty = 1
		}
		minimum := row.Deficit
		if minimum < 1 {
			return nil, ErrReplenishmentBlocked
		}
		if minimum%minOrderQty != 0 {
			minimum = ((minimum / minOrderQty) + 1) * minOrderQty
		}
		if item.Quantity < minimum || item.Quantity%minOrderQty != 0 {
			return nil, ErrReplenishmentQuantityInvalid
		}
		if currency == "" {
			currency = strings.ToUpper(strings.TrimSpace(option.Currency))
		} else if currency != strings.ToUpper(strings.TrimSpace(option.Currency)) {
			return nil, ErrReplenishmentCurrencyConflict
		}
		supplierSKU := option.SupplierSKUID
		items = append(items, CreatePurchaseOrderItemInput{
			ProductSKUID: item.ProductSKUID, SupplierSKUID: &supplierSKU,
			Quantity: item.Quantity, UnitCostMinor: option.UnitCostMinor,
		})
	}
	if len(currency) != 3 {
		return nil, ErrInvalidInput
	}

	return s.Create(ctx, tenantID, actor, CreatePurchaseOrderInput{
		IdempotencyKey: key,
		SupplierID:     in.SupplierID,
		WarehouseID:    in.WarehouseID,
		Currency:       currency,
		Remark:         remark,
		Items:          items,
	})
}

func replenishmentSupplierOptionFor(options []ReplenishmentSupplierOption, supplierID uuid.UUID, supplierSKUID *uuid.UUID) (ReplenishmentSupplierOption, bool) {
	var selected ReplenishmentSupplierOption
	found := false
	for _, option := range options {
		if option.SupplierID != supplierID {
			continue
		}
		if supplierSKUID != nil && *supplierSKUID != option.SupplierSKUID {
			continue
		}
		if found {
			return ReplenishmentSupplierOption{}, false
		}
		selected, found = option, true
	}
	return selected, found
}

func isReplenishmentConflict(err error) bool {
	return errors.Is(err, ErrReplenishmentStale) || errors.Is(err, ErrReplenishmentBlocked) || errors.Is(err, ErrReplenishmentSupplierInvalid) || errors.Is(err, ErrReplenishmentQuantityInvalid) || errors.Is(err, ErrReplenishmentCurrencyConflict)
}

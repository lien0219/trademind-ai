package salesreturn

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm"
)

func TestParseDouyinAfterSaleEventRequiresCompleteFacts(t *testing.T) {
	ev := &douyinshop.NormalizedWebhookEvent{
		EventType: "refund_success", MsgID: "evt-refund-1", TenantID: 7,
		PlatformShopID: "shop-1", InternalShopID: uuid.NewString(),
		Data: map[string]any{
			"refund_id": "refund-1", "order_id": "order-1", "status": "success",
			"refund_amount": "12.34", "currency": "cny", "updated_at": "2026-08-23T08:00:00Z",
		},
		Raw: []byte(`{"event":"refund_success"}`),
	}
	in, err := ParseDouyinAfterSaleEvent(ev)
	if err != nil {
		t.Fatal(err)
	}
	if in.RefundAmountMinor != 1234 || in.PlatformType != TypeRefundOnly || in.Currency != "CNY" || in.InternalShopID == nil {
		t.Fatalf("unexpected normalized after-sale: %#v", in)
	}
	delete(ev.Data, "refund_amount")
	if _, err := ParseDouyinAfterSaleEvent(ev); !errors.Is(err, ErrPlatformAfterSaleInvalid) {
		t.Fatalf("expected incomplete event rejection, got %v", err)
	}
}

func TestPlatformAfterSaleUpsertIsIdempotentAndReconcilesLocalFacts(t *testing.T) {
	fx := newFixture(t, 2)
	externalOrderID := "platform-order-1"
	if err := fx.db.Model(fx.order).Updates(map[string]any{
		"platform": "douyin_shop", "shop_id": fx.shop.ID, "external_order_id": externalOrderID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	local := fx.create(t, "platform-after-sale-local", TypeRefundOnly, "", 1)
	updatedAt := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	in := PlatformAfterSaleInput{
		TenantID: 7, Platform: "douyin_shop", InternalShopID: &fx.shop.ID, PlatformShopID: "platform-shop-1",
		EventID: "after-sale-event-1", EventType: "refund_success", ExternalAfterSaleID: "after-sale-1",
		ExternalOrderID: externalOrderID, PlatformType: TypeRefundOnly, PlatformStatus: "success",
		RefundAmountMinor: local.RefundAmountMinor, Currency: "CNY", PlatformUpdatedAt: &updatedAt,
		RawPayload: []byte(`{"event":"refund_success","id":"after-sale-1"}`),
	}
	if err := fx.service.UpsertPlatformAfterSale(t.Context(), in); err != nil {
		t.Fatal(err)
	}
	if err := fx.service.UpsertPlatformAfterSale(t.Context(), in); err != nil {
		t.Fatalf("same event replay failed: %v", err)
	}

	result, err := fx.service.ListPlatformAfterSales(t.Context(), PlatformAfterSaleListQuery{TenantID: 7, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.List) != 1 || result.List[0].ReconciliationStatus != PlatformReconciliationMatched || result.List[0].SalesReturnID == nil || *result.List[0].SalesReturnID != local.ID {
		t.Fatalf("unexpected reconciliation: %#v", result)
	}
	var eventCount int64
	if err := fx.db.Model(&PlatformAfterSaleEvent{}).Count(&eventCount).Error; err != nil || eventCount != 1 {
		t.Fatalf("event ledger count=%d err=%v", eventCount, err)
	}

	conflict := in
	conflict.RawPayload = []byte(`{"event":"refund_success","changed":true}`)
	if err := fx.service.UpsertPlatformAfterSale(t.Context(), conflict); !errors.Is(err, ErrPlatformEventConflict) {
		t.Fatalf("expected changed replay conflict, got %v", err)
	}

	staleAt := updatedAt.Add(-time.Hour)
	stale := in
	stale.EventID = "after-sale-event-stale"
	stale.PlatformUpdatedAt = &staleAt
	stale.PlatformStatus = "pending"
	stale.RawPayload = []byte(`{"event":"refund_pending","id":"after-sale-1"}`)
	if err := fx.service.UpsertPlatformAfterSale(t.Context(), stale); err != nil {
		t.Fatal(err)
	}
	row, err := fx.service.GetPlatformAfterSale(t.Context(), 7, result.List[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.PlatformStatus != "success" {
		t.Fatalf("stale event regressed snapshot: %#v", row)
	}
	var staleEvent PlatformAfterSaleEvent
	if err := fx.db.Where("event_id = ?", stale.EventID).First(&staleEvent).Error; err != nil || staleEvent.Applied || staleEvent.IgnoredReason != "stale_platform_update" {
		t.Fatalf("unexpected stale event ledger: %#v err=%v", staleEvent, err)
	}
	if len(row.Events) != 2 {
		t.Fatalf("expected recent event ledger in detail, got %d", len(row.Events))
	}
}

func TestPlatformAfterSaleRequiresMappedShopForOrderMatching(t *testing.T) {
	fx := newFixture(t, 1)
	orderID := fx.order.ID
	if err := fx.db.Model(fx.order).Updates(map[string]any{
		"platform": "douyin_shop", "shop_id": fx.shop.ID, "external_order_id": "mapped-order",
	}).Error; err != nil {
		t.Fatal(err)
	}
	in := PlatformAfterSaleInput{
		TenantID: 7, Platform: "douyin_shop", PlatformShopID: "unknown-shop", EventID: "unmapped-event",
		EventType: "refund_success", ExternalAfterSaleID: "unmapped-after-sale", ExternalOrderID: "mapped-order",
		PlatformType: TypeRefundOnly, PlatformStatus: "success", RefundAmountMinor: 1000, Currency: "CNY",
		RawPayload: []byte(`{"event":"refund_success"}`),
	}
	if err := fx.service.UpsertPlatformAfterSale(t.Context(), in); err != nil {
		t.Fatal(err)
	}
	row, err := fx.service.GetPlatformAfterSale(t.Context(), 7, mustPlatformAfterSaleID(t, fx.db, "unmapped-after-sale"))
	if err != nil {
		t.Fatal(err)
	}
	if row.OrderID != nil || row.ReconciliationStatus != PlatformReconciliationBlocked || row.ReconciliationReason != "local_shop_missing" {
		t.Fatalf("unmapped shop matched order %s: %#v", orderID, row)
	}
}

func TestPlatformAfterSalePersistsResolvedInternalShop(t *testing.T) {
	fx := newFixture(t, 1)
	if err := fx.db.Model(fx.order).Updates(map[string]any{
		"platform": "douyin_shop", "shop_id": fx.shop.ID, "external_order_id": "resolved-shop-order",
	}).Error; err != nil {
		t.Fatal(err)
	}
	local := fx.create(t, "resolved-shop-local", TypeRefundOnly, "", 1)
	if err := fx.service.UpsertPlatformAfterSale(t.Context(), PlatformAfterSaleInput{
		TenantID: 7, Platform: "douyin_shop", PlatformShopID: fx.shop.ExternalShopID,
		EventID: "resolved-shop-event", EventType: "refund_success", ExternalAfterSaleID: "resolved-shop-after-sale",
		ExternalOrderID: "resolved-shop-order", PlatformType: TypeRefundOnly, PlatformStatus: "success",
		RefundAmountMinor: local.RefundAmountMinor, Currency: local.Currency, RawPayload: []byte(`{"event":"refund_success"}`),
	}); err != nil {
		t.Fatal(err)
	}
	row, err := fx.service.GetPlatformAfterSale(t.Context(), 7, mustPlatformAfterSaleID(t, fx.db, "resolved-shop-after-sale"))
	if err != nil {
		t.Fatal(err)
	}
	if row.InternalShopID == nil || *row.InternalShopID != fx.shop.ID || row.ReconciliationStatus != PlatformReconciliationMatched {
		t.Fatalf("resolved internal shop was not persisted: %#v", row)
	}
}

func mustPlatformAfterSaleID(t *testing.T, db *gorm.DB, externalID string) uuid.UUID {
	t.Helper()
	var row PlatformAfterSale
	if err := db.Where("external_after_sale_id = ?", externalID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

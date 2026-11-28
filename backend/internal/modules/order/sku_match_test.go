package order

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"gorm.io/gorm"
)

func TestMatchOrderItemUsesCurrentSKUSchemaAndTenant(t *testing.T) {
	orders, _, c, row, sku, _, _ := newFulfillmentFixture(t)
	ctx := c.Request.Context()
	row.Platform = "douyin_shop"
	foreignProduct := product.Product{TenantID: row.TenantID + 1, Title: "foreign product", Source: "manual", Status: product.StatusDraft}
	require.NoError(t, orders.DB.Create(&foreignProduct).Error)
	foreignSKU := product.ProductSKU{ProductID: foreignProduct.ID, SKUCode: sku.SKUCode}
	require.NoError(t, orders.DB.Create(&foreignSKU).Error)

	match, err := orders.MatchOrderItemToSKU(ctx, row, &OrderItem{SKUCode: sku.SKUCode})
	require.NoError(t, err)
	require.Equal(t, MatchTypeLocalSKUCode, match.MatchType)
	require.Equal(t, &sku.ID, match.ProductSKUID)

	require.NoError(t, orders.DB.AutoMigrate(&productpublish.ProductPublication{}, &productpublish.ProductPublicationSKU{}))
	shopID := foreignProduct.ID // synthetic shop identity shared by both publication fixtures
	row.ShopID = &shopID
	externalID := "platform-sku"
	for _, entry := range []struct {
		tenantID int64
		sku      *product.ProductSKU
	}{
		{row.TenantID, sku},
		{foreignProduct.TenantID, &foreignSKU},
	} {
		pub := productpublish.ProductPublication{TenantID: entry.tenantID, ProductID: entry.sku.ProductID, ShopID: shopID, Platform: row.Platform, Status: "success", PublishStatus: "success"}
		require.NoError(t, orders.DB.Create(&pub).Error)
		require.NoError(t, orders.DB.Create(&productpublish.ProductPublicationSKU{
			PublicationID: pub.ID, ProductSKUID: &entry.sku.ID, ExternalSKUID: externalID, SKUCode: "seller-code",
		}).Error)
	}
	for _, tc := range []struct {
		name      string
		item      OrderItem
		matchType string
	}{
		{"external identifier", OrderItem{ExternalSKUID: &externalID}, MatchTypePublicationSKUExternalID},
		{"publication code", OrderItem{SKUCode: "seller-code"}, MatchTypePublicationSKUCode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match, err := orders.MatchOrderItemToSKU(ctx, row, &tc.item)
			require.NoError(t, err)
			require.Equal(t, tc.matchType, match.MatchType)
			require.Equal(t, &sku.ID, match.ProductSKUID)
		})
	}
}

func TestManualSKUBindingUsesCurrentSchemaAndProtectsInventoryFacts(t *testing.T) {
	orders, inv, c, row, sku, warehouseRow, _ := newFulfillmentFixture(t)
	ctx := c.Request.Context()
	require.NoError(t, orders.DB.AutoMigrate(&OrderItemSKUMatch{}))
	var item OrderItem
	require.NoError(t, orders.DB.First(&item, "order_id = ?", row.ID).Error)
	require.NoError(t, orders.DB.Model(&item).Updates(map[string]any{"product_id": nil, "product_sku_id": nil}).Error)
	foreignProduct := product.Product{TenantID: row.TenantID + 1, Title: "foreign product", Source: "manual", Status: product.StatusDraft}
	require.NoError(t, orders.DB.Create(&foreignProduct).Error)
	foreignSKU := product.ProductSKU{ProductID: foreignProduct.ID, SKUCode: "FOREIGN-SKU"}
	require.NoError(t, orders.DB.Create(&foreignSKU).Error)
	in := BindOrderItemSKUInput{OrderItemID: item.ID, ProductSKUID: foreignSKU.ID, TenantID: &row.TenantID}
	_, err := orders.BindOrderItemSKU(ctx, in, nil)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	in.ProductSKUID = sku.ID
	bound, err := orders.BindOrderItemSKU(ctx, in, nil)
	require.NoError(t, err)
	require.Equal(t, &sku.ID, bound.ProductSKUID)
	matches, err := orders.ListSKUMatchRowsForOrder(c, row.ID)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, sku.SKUCode, matches[0].LocalSKUCode)

	_, err = inv.DeductInventoryForOrder(ctx, row.ID, inventory.OrderInventoryOptions{WarehouseID: &warehouseRow.ID, TenantID: &row.TenantID})
	require.NoError(t, err)
	replacement := product.ProductSKU{ProductID: sku.ProductID, SKUCode: "REPLACEMENT-SKU"}
	require.NoError(t, orders.DB.Create(&replacement).Error)
	in.ProductSKUID = replacement.ID
	_, err = orders.BindOrderItemSKU(ctx, in, nil)
	require.ErrorIs(t, err, errOrderItemSKUInventoryLocked)

	// An automatic rematch using a different platform code has the same ledger guard.
	require.NoError(t, orders.DB.Model(row).Update("platform", "douyin_shop").Error)
	require.NoError(t, orders.DB.Model(&item).Update("sku_code", replacement.SKUCode).Error)
	summary, err := orders.MatchOrderItemsForOrder(ctx, row.ID, MatchOrderItemsOptions{Force: true})
	require.NoError(t, err)
	require.Empty(t, summary.Errors)
	require.Equal(t, 1, summary.Preserved)
	require.NoError(t, orders.DB.First(&item, "id = ?", item.ID).Error)
	require.Equal(t, &sku.ID, item.ProductSKUID)
}

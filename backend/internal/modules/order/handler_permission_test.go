package order

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func TestOrderWriteHandlersRejectReadonlyPrincipals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_permission_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}); err != nil {
		t.Fatal(err)
	}
	actorID := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: actorID},
		TenantID:     1,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test",
		Role:         adminperm.RoleReadonly,
		Status:       admin.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	orderID := uuid.New()
	itemID := uuid.New()
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/orders"},
		{http.MethodPut, "/api/v1/orders/" + orderID.String()},
		{http.MethodDelete, "/api/v1/orders/" + orderID.String()},
		{http.MethodPost, "/api/v1/orders/" + orderID.String() + "/items"},
		{http.MethodPut, "/api/v1/orders/" + orderID.String() + "/items/" + itemID.String()},
		{http.MethodDelete, "/api/v1/orders/" + orderID.String() + "/items/" + itemID.String()},
		{http.MethodPut, "/api/v1/orders/" + orderID.String() + "/shipments/" + itemID.String()},
		{http.MethodDelete, "/api/v1/orders/" + orderID.String() + "/shipments/" + itemID.String()},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("readonly order write must be rejected: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOrderReadHandlersRequireOrderView(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_read_permission_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}); err != nil {
		t.Fatal(err)
	}
	actorID := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: actorID},
		TenantID:     1,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test",
		Role:         adminperm.RoleReviewer,
		Status:       admin.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(1))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}, Inv: &inventory.Service{DB: db}})

	orderID := uuid.NewString()
	cases := []string{
		"/api/v1/orders",
		"/api/v1/orders/" + orderID,
		"/api/v1/orders/" + orderID + "/inventory-effects",
		"/api/v1/orders/" + orderID + "/sku-matches",
		"/api/v1/order-item-sku-matches",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("order read without order.view must be rejected: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOrderCreateReturnsRecoverableConflictWhenInventoryFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_create_inventory_conflict_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{}, &admin.UserStorePermission{}, &warehouse.Warehouse{}, &product.Product{}, &product.ProductSKU{},
		&Order{}, &OrderItem{}, &OrderShipment{}, &inventory.WarehouseStockBalance{}, &inventory.InventoryMovement{},
		&inventory.InventoryChangeLog{}, &inventory.OrderInventoryEffect{},
	); err != nil {
		t.Fatal(err)
	}
	actorID, warehouseID, productID, skuID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: actorID},
		TenantID:     1,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test",
		Role:         adminperm.RoleOperator,
		Status:       admin.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&warehouse.Warehouse{
		Base: model.Base{ID: warehouseID}, TenantID: 1, Code: "ORDER-CONFLICT", Name: "Order conflict", Status: warehouse.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&product.Product{
		Base: model.Base{ID: productID}, TenantID: 1, Source: "manual", Title: "Order conflict product", Status: product.StatusDraft,
	}).Error; err != nil {
		t.Fatal(err)
	}
	stock := 0
	if err := db.Create(&product.ProductSKU{
		HardDeleteBase: model.HardDeleteBase{ID: skuID}, ProductID: productID, SKUCode: "ORDER-CONFLICT-SKU", Stock: &stock,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&inventory.WarehouseStockBalance{
		TenantID: 1, WarehouseID: warehouseID, ProductSKUID: skuID, OnHand: 0, Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	warehouseSvc := &warehouse.Service{DB: db}
	invSvc := &inventory.Service{DB: db, Warehouses: warehouseSvc}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(1))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{
		Svc: &Service{DB: db, Warehouses: warehouseSvc},
		Inv: invSvc,
	})

	body := fmt.Sprintf(`{
		"platform":"manual","warehouseId":"%s","orderNo":"ORDER-CONFLICT-1","customerName":"Buyer",
		"status":"paid","paymentStatus":"paid","fulfillmentStatus":"unfulfilled","currency":"CNY",
		"deductInventory":true,"items":[{"productId":"%s","productSkuId":"%s","productTitle":"Conflict item","quantity":2}]
	}`, warehouseID, productID, skuID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("inventory failure after order creation must be an explicit conflict: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data, ok := envelope.Data.(map[string]any)
	if !ok || data["orderId"] == "" || data["order"] == nil || data["inventoryDeduction"] == nil {
		t.Fatalf("partial create response must expose recoverable order context: %#v", envelope.Data)
	}
	var orderCount, failedEffectCount int64
	if err := db.Model(&Order{}).Where("order_no = ?", "ORDER-CONFLICT-1").Count(&orderCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&inventory.OrderInventoryEffect{}).Where("status = ?", inventory.InventoryEffectFailed).Count(&failedEffectCount).Error; err != nil {
		t.Fatal(err)
	}
	if orderCount != 1 || failedEffectCount != 1 {
		t.Fatalf("expected one persisted order and one retryable failed inventory effect, orders=%d failedEffects=%d", orderCount, failedEffectCount)
	}
}

func TestOrderUpdateRejectsReplacingItemsAfterInventoryEffect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_locked_items_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{}, &admin.UserStorePermission{}, &Order{}, &OrderItem{}, &OrderShipment{}, &inventory.OrderInventoryEffect{},
	); err != nil {
		t.Fatal(err)
	}
	actorID, orderID, itemID, skuID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: actorID},
		TenantID:     1,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test",
		Role:         adminperm.RoleOperator,
		Status:       admin.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Order{
		Base:              model.Base{ID: orderID},
		TenantID:          1,
		Platform:          "manual",
		OrderNo:           "ORDER-LOCKED-1",
		CustomerName:      "locked",
		Status:            "paid",
		PaymentStatus:     "paid",
		FulfillmentStatus: "unfulfilled",
		Currency:          "CNY",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&inventory.OrderInventoryEffect{
		TenantID: 1, OrderID: orderID, OrderItemID: itemID, ProductSKUID: skuID,
		EffectType: inventory.EffectTypeReserve, Quantity: 1, Status: inventory.InventoryEffectSuccess,
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(1))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}, Inv: &inventory.Service{DB: db}})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/orders/"+orderID.String(), strings.NewReader(`{"replaceItems":true,"items":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("inventory-locked order item replacement must be rejected: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOrderInventoryWritesCannotCrossTenantBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_inventory_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &Order{}, &OrderItem{}, &OrderShipment{}, &inventory.OrderInventoryEffect{}); err != nil {
		t.Fatal(err)
	}
	actorID, orderID := uuid.New(), uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: 2, Username: admin.NewInternalUsername(), PasswordHash: "test",
		Role: adminperm.RoleOperator, Status: admin.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Order{
		Base: model.Base{ID: orderID}, TenantID: 1, Platform: "manual", OrderNo: "TENANT-BOUNDARY-1", CustomerName: "Buyer",
		Status: StatusPaid, PaymentStatus: PaymentPaid, FulfillmentStatus: FulfillmentUnfulfilled, Currency: "CNY",
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(2))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}, Inv: &inventory.Service{DB: db}})

	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/orders/" + orderID.String()},
		{http.MethodPost, "/api/v1/orders/" + orderID.String() + "/deduct-inventory"},
		{http.MethodPost, "/api/v1/orders/" + orderID.String() + "/restore-inventory"},
	}
	for _, tc := range paths {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("cross-tenant order access must be hidden: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestGlobalSKUMatchesAreTenantScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_sku_match_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Order{}, &OrderItem{}, &OrderItemSKUMatch{}); err != nil {
		t.Fatal(err)
	}
	orderA, orderB := uuid.New(), uuid.New()
	for _, row := range []Order{
		{Base: model.Base{ID: orderA}, TenantID: 1, Platform: "manual", OrderNo: "MATCH-T1", CustomerName: "A", Status: StatusPending, PaymentStatus: PaymentUnpaid, FulfillmentStatus: FulfillmentUnfulfilled, Currency: "CNY"},
		{Base: model.Base{ID: orderB}, TenantID: 2, Platform: "manual", OrderNo: "MATCH-T2", CustomerName: "B", Status: StatusPending, PaymentStatus: PaymentUnpaid, FulfillmentStatus: FulfillmentUnfulfilled, Currency: "CNY"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []OrderItemSKUMatch{
		{Base: model.Base{ID: uuid.New()}, OrderID: orderA, OrderItemID: uuid.New(), Platform: "manual", MatchType: MatchTypeManual, MatchStatus: MatchStatusManualBound},
		{Base: model.Base{ID: uuid.New()}, OrderID: orderB, OrderItemID: uuid.New(), Platform: "manual", MatchType: MatchTypeManual, MatchStatus: MatchStatusManualBound},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/order-item-sku-matches", nil)
	c.Set(ctxkey.TenantID, int64(1))
	rows, total, err := (&Service{DB: db}).ListSKUMatchGlobal(c, SKUMatchListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].OrderID != orderA {
		t.Fatalf("global SKU matches must remain tenant-scoped: total=%d rows=%#v", total, rows)
	}
}

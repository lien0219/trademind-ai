package productcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestDouyinRequiredAttributeMissingBlocksReadiness(t *testing.T) {
	db := newDouyinCheckTestDB(t)
	prodID, shopID := seedDouyinCheckBase(t, db)
	now := time.Now().UTC()
	if err := db.Create(&shop.PlatformCategory{
		Platform:   "douyin_shop",
		CategoryID: "leaf-1",
		ParentID:   "root",
		Name:       "叶子类目",
		IsLeaf:     true,
		SyncedAt:   &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&shop.PlatformCategoryAttribute{
		Platform:   "douyin_shop",
		CategoryID: "leaf-1",
		AttrID:     "brand",
		Name:       "品牌",
		Required:   true,
		SyncedAt:   &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfgAttrs, _ := json.Marshal(map[string]any{})
	if err := db.Create(&product.ProductPlatformPublishConfig{
		ProductID:          prodID,
		Platform:           "douyin_shop",
		ShopID:             &shopID,
		CategoryID:         "leaf-1",
		CategoryPath:       "叶子类目",
		PlatformAttributes: datatypes.JSON(cfgAttrs),
	}).Error; err != nil {
		t.Fatal(err)
	}
	platformdouyin.RegisterProvider()
	svc := &Service{DB: db, Settings: &settings.Service{DB: db}, Shops: &shop.Service{DB: db}}
	res, err := svc.CheckProductReadiness(readinessTenantContext(1), CheckProductReadinessRequest{
		ProductID: prodID,
		Platform:  "douyin_shop",
		ShopID:    &shopID,
	})
	if err != nil {
		t.Fatalf("CheckProductReadiness() error = %v", err)
	}
	if !hasCheckCode(res.Checks, shop.DouyinRequiredAttrMissing) || res.CanPublish {
		t.Fatalf("expected required attr missing to block readiness: %+v", res.Checks)
	}
}

func TestDouyinNonLeafCategoryBlocksReadiness(t *testing.T) {
	db := newDouyinCheckTestDB(t)
	prodID, shopID := seedDouyinCheckBase(t, db)
	now := time.Now().UTC()
	if err := db.Create(&shop.PlatformCategory{
		Platform:   "douyin_shop",
		CategoryID: "parent-1",
		Name:       "父类目",
		IsLeaf:     false,
		SyncedAt:   &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfgAttrs, _ := json.Marshal(map[string]any{"brand": "ok"})
	if err := db.Create(&product.ProductPlatformPublishConfig{
		ProductID:          prodID,
		Platform:           "douyin_shop",
		ShopID:             &shopID,
		CategoryID:         "parent-1",
		CategoryPath:       "父类目",
		PlatformAttributes: datatypes.JSON(cfgAttrs),
	}).Error; err != nil {
		t.Fatal(err)
	}
	platformdouyin.RegisterProvider()
	svc := &Service{DB: db, Settings: &settings.Service{DB: db}, Shops: &shop.Service{DB: db}}
	res, err := svc.CheckProductReadiness(readinessTenantContext(1), CheckProductReadinessRequest{
		ProductID: prodID,
		Platform:  "douyin_shop",
		ShopID:    &shopID,
	})
	if err != nil {
		t.Fatalf("CheckProductReadiness() error = %v", err)
	}
	if !hasCheckCode(res.Checks, shop.DouyinCategoryNotLeaf) || res.CanPublish {
		t.Fatalf("expected non-leaf category to block readiness: %+v", res.Checks)
	}
}

func TestDouyinSavedMappingBlocksReadiness(t *testing.T) {
	db := newDouyinCheckTestDB(t)
	prodID, shopID := seedDouyinCheckBase(t, db)
	now := time.Now().UTC()
	if err := db.Create(&shop.PlatformCategory{
		Platform:   "douyin_shop",
		CategoryID: "leaf-1",
		Name:       "Leaf",
		IsLeaf:     true,
		SyncedAt:   &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&product.ProductPlatformPublishConfig{
		ProductID:    prodID,
		Platform:     "douyin_shop",
		ShopID:       &shopID,
		CategoryID:   "leaf-1",
		CategoryPath: "Leaf",
	}).Error; err != nil {
		t.Fatal(err)
	}
	platformdouyin.RegisterProvider()
	svc := &Service{DB: db, Settings: &settings.Service{DB: db}, Shops: &shop.Service{DB: db}}
	res, err := svc.CheckProductReadiness(readinessTenantContext(1), CheckProductReadinessRequest{
		ProductID: prodID,
		Platform:  "douyin_shop",
		ShopID:    &shopID,
	})
	if err != nil {
		t.Fatalf("CheckProductReadiness() error = %v", err)
	}
	for _, code := range []string{product.DouyinTitleMissing, product.DouyinMainImageMissing, product.DouyinSKUMissing} {
		if !hasCheckCode(res.Checks, code) {
			t.Fatalf("expected saved mapping check %s in readiness: %+v", code, res.Checks)
		}
	}
	if res.CanPublish {
		t.Fatalf("expected saved mapping validation to block readiness: %+v", res.Checks)
	}
}

func newDouyinCheckTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&product.ProductPlatformPublishConfig{},
		&shop.Shop{},
		&shop.ShopAuthToken{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
		&settings.Setting{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]settings.Setting{
		{GroupKey: "platform_douyin_shop", ItemKey: "app_key", ItemValue: "app-key"},
		{GroupKey: "platform_douyin_shop", ItemKey: "app_secret", ItemValue: "app-secret"},
		{GroupKey: "platform_douyin_shop", ItemKey: "redirect_uri", ItemValue: "https://example.com/cb"},
		{GroupKey: "platform_douyin_shop", ItemKey: "timeout_sec", ItemValue: "30"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func seedDouyinCheckBase(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	prod := product.Product{TenantID: 1, Source: "manual", Title: "Demo", Description: "Long enough description", Currency: "CNY", Status: product.StatusReady}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&product.ProductImage{ProductID: prod.ID, ImageType: product.ImageTypeMain, PublicURL: "https://example.com/a.jpg"}).Error; err != nil {
		t.Fatal(err)
	}
	price := 10.0
	stock := 3
	if err := db.Create(&product.ProductSKU{ProductID: prod.ID, SKUName: "默认", Price: &price, Stock: &stock}).Error; err != nil {
		t.Fatal(err)
	}
	row := shop.Shop{Platform: "douyin_shop", ShopName: "抖店", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&row).Update("tenant_id", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&shop.ShopAuthToken{ShopID: row.ID, Platform: "douyin_shop", AuthType: "oauth2", AccessTokenEnc: "masked"}).Error; err != nil {
		t.Fatal(err)
	}
	return prod.ID, row.ID
}

func hasCheckCode(items []CheckItem, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}

func TestCheckProductReadinessRejectsCrossTenantProduct(t *testing.T) {
	db := newDouyinCheckTestDB(t)
	prod := product.Product{TenantID: 2, Source: "manual", Title: "tenant two private title", Description: "private", Currency: "CNY", Status: product.StatusReady}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	if _, err := svc.CheckProductReadiness(readinessTenantContext(1), CheckProductReadinessRequest{ProductID: prod.ID}); err == nil {
		t.Fatal("expected tenant 1 readiness request for tenant 2 product to fail")
	}
}

func TestCheckProductReadinessRejectsCrossTenantShop(t *testing.T) {
	db := newDouyinCheckTestDB(t)
	prod := product.Product{TenantID: 1, Source: "manual", Title: "Demo", Description: "Description", Currency: "CNY", Status: product.StatusReady}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	foreignShop := shop.Shop{TenantID: 2, Platform: "douyin_shop", ShopName: "foreign", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	if err := db.Create(&foreignShop).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	if _, err := svc.CheckProductReadiness(readinessTenantContext(1), CheckProductReadinessRequest{ProductID: prod.ID, ShopID: &foreignShop.ID}); err == nil {
		t.Fatal("expected foreign shop to be rejected")
	}
}

func TestBatchReadinessDoesNotReturnCrossTenantProduct(t *testing.T) {
	db := newDouyinCheckTestDB(t)
	ownShop := shop.Shop{TenantID: 1, Platform: "douyin_shop", ShopName: "own", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	if err := db.Create(&ownShop).Error; err != nil {
		t.Fatal(err)
	}
	foreignProduct := product.Product{TenantID: 2, Source: "manual", Title: "tenant two private title", Description: "private", Currency: "CNY", Status: product.StatusReady}
	if err := db.Create(&foreignProduct).Error; err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"productIds": []string{foreignProduct.ID.String()},
		"platform":   "douyin_shop",
		"shopId":     ownShop.ID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/readiness/batch", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request = c.Request.WithContext(readinessTenantContext(1))
	(&Handler{Svc: &Service{DB: db}}).BatchReadiness(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected cross-tenant batch readiness to return 404, got=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), foreignProduct.ID.String()) || strings.Contains(w.Body.String(), "tenant two private title") {
		t.Fatalf("cross-tenant product data leaked in response: %s", w.Body.String())
	}
}

func readinessTenantContext(tenantID int64) context.Context {
	return security.WithTenantContext(context.Background(), security.WorkerTenantContext(tenantID, uuid.Nil))
}

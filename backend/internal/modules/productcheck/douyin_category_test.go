package productcheck

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
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
	res, err := svc.CheckProductReadiness(context.Background(), CheckProductReadinessRequest{
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
	res, err := svc.CheckProductReadiness(context.Background(), CheckProductReadinessRequest{
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
	prod := product.Product{Source: "manual", Title: "Demo", Description: "Long enough description", Currency: "CNY", Status: product.StatusReady}
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

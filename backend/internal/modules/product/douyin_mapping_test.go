package product

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestBuildDouyinDraftMappingPrefersAIFieldsAndMarksExternalImages(t *testing.T) {
	db := newDouyinMappingTestDB(t)
	svc := &Service{DB: db, Settings: &settings.Service{DB: db}}
	prod := Product{
		Source:        "1688",
		OriginalTitle: "Original Title",
		Title:         "Normal Title",
		AITitle:       " AI Better Title ",
		Description:   "<p>normal description</p>",
		AIDescription: "<div>AI Better Description</div>",
		Currency:      "CNY",
		Status:        StatusReady,
	}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	price := 120.0
	cost := 80.0
	stock := 5
	attrs, _ := json.Marshal(map[string]any{"color": "red"})
	if err := db.Create(&ProductSKU{ProductID: prod.ID, SKUName: "Red", Attrs: datatypes.JSON(attrs), Price: &price, CostPrice: &cost, Stock: &stock}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductImage{ProductID: prod.ID, ImageType: ImageTypeMain, PublicURL: "https://img.example.com/main.jpg", SortOrder: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductImage{ProductID: prod.ID, ImageType: ImageTypeDetail, PublicURL: "https://img.example.com/detail.jpg", SortOrder: 2}).Error; err != nil {
		t.Fatal(err)
	}
	cfgAttrs, _ := json.Marshal(map[string]any{"brand": "tm"})
	if err := db.Create(&ProductPlatformPublishConfig{
		ProductID:          prod.ID,
		Platform:           "douyin_shop",
		CategoryID:         "leaf-1",
		CategoryPath:       "Root / Leaf",
		PlatformAttributes: datatypes.JSON(cfgAttrs),
	}).Error; err != nil {
		t.Fatal(err)
	}

	m, err := svc.BuildDouyinDraftMapping(context.Background(), prod.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "AI Better Title" {
		t.Fatalf("expected aiTitle priority, got %q", m.Title)
	}
	if m.Description != "AI Better Description" {
		t.Fatalf("expected aiDescription priority and html stripping, got %q", m.Description)
	}
	if len(m.MainImages) != 1 || !m.MainImages[0].NeedSync || m.MainImages[0].Status != "need_sync" {
		t.Fatalf("expected external main image to need sync: %+v", m.MainImages)
	}
	if len(m.DetailImages) != 1 || !m.DetailImages[0].NeedSync {
		t.Fatalf("expected external detail image to need sync: %+v", m.DetailImages)
	}
	if len(m.SKUs) != 1 || m.SKUs[0].Price != 120 || m.SKUs[0].Stock == nil || *m.SKUs[0].Stock != 5 {
		t.Fatalf("expected sku price and stock mapping: %+v", m.SKUs)
	}
}

func TestValidateDouyinDraftMappingErrorCodes(t *testing.T) {
	stock := -1
	price := 5.0
	cost := 10.0
	m := &DouyinDraftMapping{
		Platform:     "douyin_shop",
		CategoryID:   "",
		MainImages:   nil,
		DetailImages: nil,
		Attributes: []DouyinDraftAttr{{
			AttrID:   "brand",
			Name:     "Brand",
			Required: true,
		}},
		SKUs: []DouyinDraftSKU{{
			LocalSkuID: uuid.NewString(),
			Name:       "Bad",
			Price:      0,
			Stock:      &stock,
			Attrs:      map[string]any{},
		}},
		Price: DouyinDraftPrice{Currency: "CNY", Min: &price, CostMin: &cost},
		Stock: DouyinDraftStock{Unconfirmed: true},
	}
	ApplyDouyinDraftValidation(m, 3)
	for _, code := range []string{
		DouyinTitleMissing,
		DouyinMainImageMissing,
		shop.DouyinCategoryNotSelected,
		shop.DouyinRequiredAttrMissing,
		DouyinSKUPriceInvalid,
		DouyinProfitTooLow,
		DouyinStockInvalid,
	} {
		if !hasDouyinIssue(m.Errors, code) {
			t.Fatalf("expected error code %s in %+v", code, m.Errors)
		}
	}
	if !hasDouyinIssue(m.Warnings, DouyinDetailImageEmpty) ||
		!hasDouyinIssue(m.Warnings, DouyinSKUAttrIncomplete) ||
		!hasDouyinIssue(m.Warnings, DouyinStockUnconfirmed) {
		t.Fatalf("expected warning codes in %+v", m.Warnings)
	}
}

func TestSaveReadDouyinDraftMappingAndManualEditPersists(t *testing.T) {
	db := newDouyinMappingTestDB(t)
	svc := &Service{DB: db, Settings: &settings.Service{DB: db}}
	prod := Product{Source: "manual", Title: "Original", Description: "Desc", Currency: "CNY", Status: StatusReady}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	price := 99.0
	stock := 2
	mapping := &DouyinDraftMapping{
		Platform:     "douyin_shop",
		ShopID:       uuid.NewString(),
		CategoryID:   "leaf-1",
		CategoryPath: "Root / Leaf",
		Title:        "Manual Douyin Title",
		Description:  "Manual Douyin Description",
		MainImages: []DouyinDraftImage{{
			ImageType: ImageTypeMain,
			URL:       "products/main.jpg",
			ObjectKey: "products/main.jpg",
			Status:    "ready",
		}},
		Attributes: []DouyinDraftAttr{{AttrID: "brand", Name: "Brand", Value: "TradeMind"}},
		SKUs: []DouyinDraftSKU{{
			LocalSkuID: uuid.NewString(),
			Name:       "Default",
			Attrs:      map[string]any{"spec": "default"},
			Price:      price,
			Stock:      &stock,
		}},
		Price: DouyinDraftPrice{Currency: "CNY", Min: &price, Max: &price},
		Stock: DouyinDraftStock{Total: &stock, Min: &stock},
	}
	if err := svc.SaveDouyinDraftMapping(context.Background(), prod.ID, mapping); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetDouyinDraftMapping(context.Background(), prod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Manual Douyin Title" || got.Description != "Manual Douyin Description" {
		t.Fatalf("manual mapping not persisted: %+v", got)
	}
	prod.AITitle = "New AI Title"
	if err := db.Save(&prod).Error; err != nil {
		t.Fatal(err)
	}
	gotAgain, err := svc.GetDouyinDraftMapping(context.Background(), prod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotAgain.Title != "Manual Douyin Title" {
		t.Fatalf("manual mapping should not be overwritten without build: got %q", gotAgain.Title)
	}
}

func TestDouyinMappingLogsDoNotContainTokenOrSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newDouyinMappingTestDB(t)
	svc := &Service{DB: db, Settings: &settings.Service{DB: db}, OpLog: &operationlog.Service{DB: db}}
	prod := Product{
		Source:      "custom",
		Title:       "Token safe product",
		Description: "Description",
		Currency:    "CNY",
		Status:      StatusReady,
		RawData:     datatypes.JSON([]byte(`{"access_token":"secret-token","app_secret":"secret-value"}`)),
	}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	price := 20.0
	stock := 3
	if err := db.Create(&ProductSKU{ProductID: prod.ID, SKUName: "Default", Price: &price, Stock: &stock}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductImage{ProductID: prod.ID, ImageType: ImageTypeMain, ObjectKey: "products/main.jpg"}).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	Register(r.Group("/api/v1"), &Handler{Svc: svc})
	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+prod.ID.String()+"/platform-configs/douyin_shop/build-mapping", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("build mapping status=%d body=%s", w.Code, w.Body.String())
	}
	var logs []operationlog.OperationLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 {
		t.Fatal("expected operation log")
	}
	for _, row := range logs {
		low := strings.ToLower(row.Message)
		if strings.Contains(low, "token") || strings.Contains(low, "secret") {
			t.Fatalf("operation log leaked sensitive marker: %+v", row)
		}
	}
}

func newDouyinMappingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&Product{},
		&ProductImage{},
		&ProductSKU{},
		&ProductPlatformPublishConfig{},
		&shop.Shop{},
		&shop.ShopAuthToken{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
		&settings.Setting{},
		&operationlog.OperationLog{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&settings.Setting{GroupKey: "pricing", ItemKey: "minProfit", ItemValue: "3"}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func hasDouyinIssue(items []DouyinMappingIssue, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}

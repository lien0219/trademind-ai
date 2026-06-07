package productpublish

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func newDouyinPublishTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&product.Product{},
		&product.ProductSKU{},
		&product.ProductImage{},
		&product.ProductPlatformPublishConfig{},
		&ProductPublishTask{},
		&ProductPublication{},
		&ProductPublicationSKU{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestBuildDouyinProductPayloadRejectsUnuploadedImages(t *testing.T) {
	db := newDouyinPublishTestDB(t)
	pid := uuid.New()
	images, _ := json.Marshal(map[string]any{
		"mainImages": []map[string]any{{
			"url": "https://img.example.com/a.jpg", "uploadStatus": "pending",
		}},
		"detailImages": []any{},
	})
	skus, _ := json.Marshal([]product.DouyinDraftSKU{{
		LocalSkuID: uuid.NewString(), Name: "Default", Price: 99, Stock: ptrInt(10),
	}})
	price, _ := json.Marshal(product.DouyinDraftPrice{Currency: "CNY", Min: ptrFloat(99)})
	stock, _ := json.Marshal(product.DouyinDraftStock{Total: ptrInt(10)})
	if err := db.Create(&product.ProductPlatformPublishConfig{
		ProductID: pid, Platform: "douyin_shop", CategoryID: "12345",
		MappedTitle: "Test Product", MappedImages: datatypes.JSON(images),
		MappedSKUs: datatypes.JSON(skus), MappedPrice: datatypes.JSON(price), MappedStock: datatypes.JSON(stock),
	}).Error; err != nil {
		t.Fatal(err)
	}
	res, err := BuildDouyinProductPayload(context.Background(), db, pid, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors for unuploaded main image")
	}
	found := false
	for _, e := range res.Errors {
		if e.Code == product.DouyinMainImageNotUploaded {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected DOUYIN_MAIN_IMAGE_NOT_UPLOADED, got %+v", res.Errors)
	}
}

func TestBuildDouyinProductPayloadUsesUploadedImagesNotRaw(t *testing.T) {
	db := newDouyinPublishTestDB(t)
	pid := uuid.New()
	images, _ := json.Marshal(map[string]any{
		"mainImages": []map[string]any{{
			"platformImageUrl": "https://p3-aio.ecombdimg.com/obj/test.jpg",
			"uploadStatus":     "uploaded",
		}},
		"detailImages": []any{},
	})
	skus, _ := json.Marshal([]product.DouyinDraftSKU{{
		LocalSkuID: uuid.NewString(), Name: "Red", Price: 88.8, Stock: ptrInt(3), Attrs: map[string]any{"颜色": "红"},
	}})
	price, _ := json.Marshal(product.DouyinDraftPrice{Currency: "CNY", Min: ptrFloat(88.8)})
	stock, _ := json.Marshal(product.DouyinDraftStock{Total: ptrInt(3)})
	attrs, _ := json.Marshal(map[string]any{"405": "27664"})
	if err := db.Create(&product.ProductPlatformPublishConfig{
		ProductID: pid, Platform: "douyin_shop", CategoryID: "20219",
		MappedTitle: "抖店测试商品", MappedImages: datatypes.JSON(images),
		MappedSKUs: datatypes.JSON(skus), MappedPrice: datatypes.JSON(price), MappedStock: datatypes.JSON(stock),
		PlatformAttributes: datatypes.JSON(attrs),
	}).Error; err != nil {
		t.Fatal(err)
	}
	res, err := BuildDouyinProductPayload(context.Background(), db, pid, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	if res.Payload == nil || !containsStr(res.Payload.Pic, "ecombdimg.com") {
		t.Fatalf("expected uploaded platform image url in pic: %+v", res.Payload)
	}
	if len(res.APIReq.SpecPricesV2) != 1 || res.APIReq.SpecPricesV2[0]["price"] != int64(8880) {
		t.Fatalf("expected price in fen: %+v", res.APIReq.SpecPricesV2)
	}
}

func TestBuildDouyinProductPayloadRejectsInvalidSKUPrice(t *testing.T) {
	db := newDouyinPublishTestDB(t)
	pid := uuid.New()
	images, _ := json.Marshal(map[string]any{
		"mainImages": []map[string]any{{
			"platformImageUrl": "https://p3-aio.ecombdimg.com/obj/test.jpg", "uploadStatus": "uploaded",
		}},
	})
	skus, _ := json.Marshal([]product.DouyinDraftSKU{{LocalSkuID: uuid.NewString(), Name: "Bad", Price: 0, Stock: ptrInt(1)}})
	price, _ := json.Marshal(product.DouyinDraftPrice{Currency: "CNY", Min: ptrFloat(0)})
	if err := db.Create(&product.ProductPlatformPublishConfig{
		ProductID: pid, Platform: "douyin_shop", CategoryID: "20219",
		MappedTitle: "Test", MappedImages: datatypes.JSON(images),
		MappedSKUs: datatypes.JSON(skus), MappedPrice: datatypes.JSON(price),
	}).Error; err != nil {
		t.Fatal(err)
	}
	res, err := BuildDouyinProductPayload(context.Background(), db, pid, "")
	if err != nil {
		t.Fatal(err)
	}
	if !issueHasCode(res.Errors, product.DouyinSKUPriceInvalid) {
		t.Fatalf("expected invalid sku price error, got %+v", res.Errors)
	}
}

func TestBuildDouyinProductPayloadMissingMappingConfig(t *testing.T) {
	db := newDouyinPublishTestDB(t)
	_, err := BuildDouyinProductPayload(context.Background(), db, uuid.New(), "")
	if err == nil {
		t.Fatal("expected error when mapping config missing")
	}
}

func ptrInt(v int) *int           { return &v }
func ptrFloat(v float64) *float64 { return &v }

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func issueHasCode(items []product.DouyinMappingIssue, code string) bool {
	for _, it := range items {
		if it.Code == code {
			return true
		}
	}
	return false
}

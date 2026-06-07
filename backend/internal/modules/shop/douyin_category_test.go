package shop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"
)

func TestDouyinCategorySyncWritesCacheIdempotentlyAndRefreshesToken(t *testing.T) {
	db := newDouyinCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newDouyinCategoryTestService(t, db, enc)
	api := newDouyinCategoryFakeAPI(t)
	seedDouyinSettings(t, db, api.URL)
	shopID := seedDouyinAuthorizedShop(t, db, enc, true)

	c := testGinContext()
	stat, err := svc.SyncDouyinCategories(c, shopID, nil)
	if err != nil {
		t.Fatalf("SyncDouyinCategories() error = %v", err)
	}
	if stat.Count != 3 || stat.LeafCount != 2 {
		t.Fatalf("unexpected stats: %+v", stat)
	}
	stat, err = svc.SyncDouyinCategories(c, shopID, nil)
	if err != nil {
		t.Fatalf("second SyncDouyinCategories() error = %v", err)
	}
	var count int64
	if err := db.Model(&PlatformCategory{}).Where("platform = ?", douyinPlatform).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("duplicate sync inserted rows, got %d", count)
	}
	if !api.seenRefresh {
		t.Fatalf("expected expired token to be refreshed before category sync")
	}
	var tok ShopAuthToken
	if err := db.First(&tok, "shop_id = ?", shopID).Error; err != nil {
		t.Fatal(err)
	}
	access, _ := enc.Decrypt(tok.AccessTokenEnc)
	if string(access) != "new-access" {
		t.Fatalf("refreshed access token was not persisted")
	}
}

func TestDouyinCategorySyncRequiresAuthorizedShop(t *testing.T) {
	db := newDouyinCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newDouyinCategoryTestService(t, db, enc)
	api := newDouyinCategoryFakeAPI(t)
	seedDouyinSettings(t, db, api.URL)
	shopID := seedDouyinUnauthorizedShop(t, db)
	if _, err := svc.SyncDouyinCategories(testGinContext(), shopID, nil); err == nil {
		t.Fatalf("expected unauthorized shop sync failure")
	}
}

func TestDouyinCategoryAttributeSyncWritesCache(t *testing.T) {
	db := newDouyinCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newDouyinCategoryTestService(t, db, enc)
	api := newDouyinCategoryFakeAPI(t)
	seedDouyinSettings(t, db, api.URL)
	shopID := seedDouyinAuthorizedShop(t, db, enc, false)
	out, err := svc.SyncDouyinCategoryAttributes(testGinContext(), shopID, "300", nil)
	if err != nil {
		t.Fatalf("SyncDouyinCategoryAttributes() error = %v", err)
	}
	if len(out) != 2 || !out[0].Required {
		t.Fatalf("unexpected attributes: %+v", out)
	}
	var count int64
	if err := db.Model(&PlatformCategoryAttribute{}).Where("platform = ? AND category_id = ?", douyinPlatform, "300").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 cached attrs, got %d", count)
	}
}

func TestDouyinCategoryLogsDoNotContainSecrets(t *testing.T) {
	db := newDouyinCategoryTestDB(t)
	enc, _ := encrypt.NewService("test-master-key")
	svc := newDouyinCategoryTestService(t, db, enc)
	api := newDouyinCategoryFakeAPI(t)
	seedDouyinSettings(t, db, api.URL)
	shopID := seedDouyinAuthorizedShop(t, db, enc, false)
	if _, err := svc.SyncDouyinCategories(testGinContext(), shopID, nil); err != nil {
		t.Fatalf("SyncDouyinCategories() error = %v", err)
	}
	var logs []operationlog.OperationLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(logs)
	low := strings.ToLower(string(blob))
	for _, secret := range []string{"old-access", "new-access", "old-refresh", "new-refresh", "app-secret"} {
		if strings.Contains(low, secret) {
			t.Fatalf("operation logs leaked secret %q: %s", secret, string(blob))
		}
	}
}

type douyinCategoryFakeAPI struct {
	*httptest.Server
	seenRefresh bool
}

func newDouyinCategoryFakeAPI(t *testing.T) *douyinCategoryFakeAPI {
	t.Helper()
	api := &douyinCategoryFakeAPI{}
	api.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		body, _ := io.ReadAll(r.Body)
		var params map[string]any
		_ = json.Unmarshal(body, &params)
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "token.refresh":
			api.seenRefresh = true
			_, _ = w.Write([]byte(`{"code":10000,"data":{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"refresh_expires_in":7200,"shop_id":"dy-shop","shop_name":"Demo"}}`))
		case "shop.getShopCategory":
			switch strings.TrimSpace(stringValue(params["cid"])) {
			case "0", "":
				_, _ = w.Write([]byte(`{"code":10000,"data":{"category_list":[{"cid":"100","parent_id":"0","name":"服饰","level":1,"is_leaf":false},{"cid":"300","parent_id":"0","name":"配饰","level":1,"is_leaf":true}]}}`))
			case "100":
				_, _ = w.Write([]byte(`{"code":10000,"data":{"category_list":[{"cid":"200","parent_id":"100","name":"女装","level":2,"is_leaf":true}]}}`))
			default:
				_, _ = w.Write([]byte(`{"code":10000,"data":{"category_list":[]}}`))
			}
		case "product.getCatePropertyV2":
			_, _ = w.Write([]byte(`{"code":10000,"data":{"property_list":[{"property_id":"brand","property_name":"品牌","required":1,"options":[{"value_id":"1","value_name":"无品牌"}]},{"property_id":"material","property_name":"材质","required":0}]}}`))
		default:
			t.Fatalf("unexpected method %s path %s", method, r.URL.Path)
		}
	}))
	t.Cleanup(api.Close)
	return api
}

func newDouyinCategoryTestDB(t *testing.T) *gorm.DB {
	db := newDouyinShopTestDB(t)
	if err := db.AutoMigrate(&settings.Setting{}, &operationlog.OperationLog{}, &PlatformCategory{}, &PlatformCategoryAttribute{}); err != nil {
		t.Fatalf("migrate category test tables: %v", err)
	}
	return db
}

func newDouyinCategoryTestService(t *testing.T, db *gorm.DB, enc *encrypt.Service) *Service {
	return &Service{
		DB:        db,
		Encrypter: enc,
		Settings:  &settings.Service{DB: db, Encrypter: enc},
		OpLog:     &operationlog.Service{DB: db},
	}
}

func seedDouyinSettings(t *testing.T, db *gorm.DB, apiURL string) {
	rows := []settings.Setting{
		{GroupKey: "platform_douyin_shop", ItemKey: "app_key", ItemValue: "app-key"},
		{GroupKey: "platform_douyin_shop", ItemKey: "app_secret", ItemValue: "app-secret"},
		{GroupKey: "platform_douyin_shop", ItemKey: "service_id", ItemValue: "svc"},
		{GroupKey: "platform_douyin_shop", ItemKey: "redirect_uri", ItemValue: "https://example.com/cb"},
		{GroupKey: "platform_douyin_shop", ItemKey: "api_base_url", ItemValue: apiURL},
		{GroupKey: "platform_douyin_shop", ItemKey: "timeout_sec", ItemValue: "30"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed settings: %v", err)
	}
}

func seedDouyinAuthorizedShop(t *testing.T, db *gorm.DB, enc *encrypt.Service, expiredAccess bool) uuid.UUID {
	shop := Shop{Platform: douyinPlatform, ShopName: "Demo", Status: StatusActive, AuthStatus: AuthAuthorized}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatal(err)
	}
	access, _ := enc.Encrypt([]byte("old-access"))
	refresh, _ := enc.Encrypt([]byte("old-refresh"))
	exp := time.Now().Add(time.Hour)
	if expiredAccess {
		exp = time.Now().Add(-time.Minute)
	}
	rexp := time.Now().Add(24 * time.Hour)
	tok := ShopAuthToken{ShopID: shop.ID, Platform: douyinPlatform, AuthType: "oauth2", AccessTokenEnc: access, RefreshTokenEnc: refresh, ExpiresAt: &exp, RefreshExpiresAt: &rexp}
	if err := db.Create(&tok).Error; err != nil {
		t.Fatal(err)
	}
	return shop.ID
}

func seedDouyinUnauthorizedShop(t *testing.T, db *gorm.DB) uuid.UUID {
	shop := Shop{Platform: douyinPlatform, ShopName: "Demo", Status: StatusActive, AuthStatus: AuthNeedCheck}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatal(err)
	}
	return shop.ID
}

func testGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c
}

func stringValue(v any) string {
	return strings.TrimSpace(fmt.Sprint(v))
}

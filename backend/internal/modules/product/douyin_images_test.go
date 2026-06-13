package product

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/safedownload"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm"
)

type stubDouyinUploader struct {
	calls int
	err   error
}

func (s *stubDouyinUploader) UploadImage(ctx context.Context, shopID string, req platformdouyin.UploadImageRequest) (*platformdouyin.PlatformImage, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return &platformdouyin.PlatformImage{
		ImageID: "material-" + req.ImageType,
		URL:     "https://douyin.example.com/" + req.FileName,
		Raw:     map[string]any{"ok": true},
	}, nil
}

func seedDouyinPlatformSettings(t *testing.T, db *gorm.DB) {
	t.Helper()
	for k, v := range map[string]string{
		"app_key": "test-key", "app_secret": "test-secret", "redirect_uri": "https://example.com/cb",
		"timeout_sec": "30", "real_api_enabled": "true", "product_publish_enabled": "true",
		"platform_runtime_status": "normal",
	} {
		if err := db.Create(&settings.Setting{GroupKey: "platform_douyin_shop", ItemKey: k, ItemValue: v, ValueType: "string"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	platformdouyin.BindShops(&stubDouyinBridge{settings: db})
}

type stubDouyinBridge struct {
	settings *gorm.DB
}

func (b *stubDouyinBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	return nil
}
func (b *stubDouyinBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	return nil
}
func (b *stubDouyinBridge) DouyinGlobalSettings(ctx context.Context) (map[string]string, error) {
	var rows []settings.Setting
	if err := b.settings.WithContext(ctx).Where("group_key = ?", "platform_douyin_shop").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.ItemKey] = r.ItemValue
	}
	return out, nil
}

func TestDouyinImageUploadReadsStorageAndPersistsPlatformImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newDouyinMappingTestDB(t)
	seedDouyinPlatformSettings(t, db)
	root := t.TempDir()
	seedLocalStorage(t, db, root, "https://cdn.example.test/static")
	settingsSvc := &settings.Service{DB: db}
	filesSvc := &files.Service{DB: db, Settings: settingsSvc}
	uploader := &stubDouyinUploader{}
	svc := &Service{DB: db, Settings: settingsSvc, DouyinImageUploader: uploader}
	prod := Product{Source: "manual", Title: "Demo", Description: "Desc", Currency: "CNY", Status: StatusReady}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	data := testPNG(t)
	rec, err := filesSvc.SaveProcessed(context.Background(), files.SaveProcessedOpts{
		OriginalName: "main.png",
		ObjectKey:    "products/main.png",
		Data:         data,
		ContentType:  "image/png",
	})
	if err != nil {
		t.Fatal(err)
	}
	price := 10.0
	stock := 1
	shopID := uuid.NewString()
	mapping := validDouyinMapping(shopID, price, stock)
	mapping.MainImages = []DouyinDraftImage{{
		LocalImageID: uuid.NewString(),
		ImageType:    ImageTypeMain,
		URL:          rec.PublicURL,
		StorageURL:   rec.PublicURL,
		StorageKey:   rec.ObjectKey,
		ObjectKey:    rec.ObjectKey,
		UploadStatus: DouyinImageStatusPending,
	}}
	if err := svc.SaveDouyinDraftMapping(context.Background(), prod.ID, mapping); err != nil {
		t.Fatal(err)
	}
	out, err := svc.UploadDouyinImages(testGinContext(), prod.ID, DouyinImageUploadBody{ImageTypes: []string{"main"}}, nil, filesSvc)
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary.Uploaded != 1 || uploader.calls != 1 {
		t.Fatalf("expected one upload, summary=%+v calls=%d", out.Summary, uploader.calls)
	}
	got := out.Mapping.MainImages[0]
	if got.PlatformImageID == "" || got.PlatformImageURL == "" || got.UploadStatus != DouyinImageStatusUploaded {
		t.Fatalf("platform image fields not persisted: %+v", got)
	}
	out2, err := svc.UploadDouyinImages(testGinContext(), prod.ID, DouyinImageUploadBody{ImageTypes: []string{"main"}}, nil, filesSvc)
	if err != nil {
		t.Fatal(err)
	}
	if uploader.calls != 1 || out2.Summary.Uploaded != 1 {
		t.Fatalf("uploaded image should not repeat, summary=%+v calls=%d", out2.Summary, uploader.calls)
	}
}

func TestDouyinImageUploadFailureAndRetry(t *testing.T) {
	db := newDouyinMappingTestDB(t)
	seedDouyinPlatformSettings(t, db)
	root := t.TempDir()
	seedLocalStorage(t, db, root, "https://cdn.example.test/static")
	settingsSvc := &settings.Service{DB: db}
	filesSvc := &files.Service{DB: db, Settings: settingsSvc}
	prod := Product{Source: "manual", Title: "Demo", Description: "Desc", Currency: "CNY", Status: StatusReady}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	rec, err := filesSvc.SaveProcessed(context.Background(), files.SaveProcessedOpts{
		OriginalName: "detail.png",
		ObjectKey:    "products/detail.png",
		Data:         testPNG(t),
		ContentType:  "image/png",
	})
	if err != nil {
		t.Fatal(err)
	}
	price := 10.0
	stock := 1
	imgID := uuid.NewString()
	mapping := validDouyinMapping(uuid.NewString(), price, stock)
	mapping.MainImages[0].PlatformImageID = "already-ok"
	mapping.MainImages[0].UploadStatus = DouyinImageStatusUploaded
	mapping.DetailImages = []DouyinDraftImage{{
		LocalImageID: imgID,
		ImageType:    ImageTypeDetail,
		URL:          rec.PublicURL,
		StorageURL:   rec.PublicURL,
		StorageKey:   rec.ObjectKey,
		ObjectKey:    rec.ObjectKey,
		UploadStatus: DouyinImageStatusPending,
	}}
	uploader := &stubDouyinUploader{err: platformdouyin.NewError(platformdouyin.CodeDouyinRateLimited, "rate limited", "429", "rate", "")}
	svc := &Service{DB: db, Settings: settingsSvc, DouyinImageUploader: uploader}
	if err := svc.SaveDouyinDraftMapping(context.Background(), prod.ID, mapping); err != nil {
		t.Fatal(err)
	}
	out, err := svc.UploadDouyinImages(testGinContext(), prod.ID, DouyinImageUploadBody{ImageTypes: []string{"detail"}}, nil, filesSvc)
	if err != nil {
		t.Fatal(err)
	}
	if out.Mapping.DetailImages[0].UploadStatus != DouyinImageStatusFailed ||
		out.Mapping.DetailImages[0].ErrorCode != platformdouyin.CodeDouyinRateLimited {
		t.Fatalf("expected failed detail image with code: %+v", out.Mapping.DetailImages[0])
	}
	uploader.err = nil
	out, err = svc.RetryDouyinImage(testGinContext(), prod.ID, imgID, nil, filesSvc)
	if err != nil {
		t.Fatal(err)
	}
	if out.Mapping.DetailImages[0].UploadStatus != DouyinImageStatusUploaded {
		t.Fatalf("retry should upload detail image: %+v", out.Mapping.DetailImages[0])
	}
}

func TestDouyinValidationRequiresMainImageUpload(t *testing.T) {
	price := 10.0
	stock := 1
	m := validDouyinMapping(uuid.NewString(), price, stock)
	m.MainImages[0].PlatformImageID = ""
	m.MainImages[0].UploadStatus = DouyinImageStatusPending
	ApplyDouyinDraftValidation(m, 0)
	if !hasDouyinIssue(m.Errors, DouyinMainImageNotUploaded) {
		t.Fatalf("expected main image not uploaded error: %+v", m.Errors)
	}
	m.MainImages[0].PlatformImageID = "material-main"
	m.MainImages[0].UploadStatus = DouyinImageStatusUploaded
	ApplyDouyinDraftValidation(m, 0)
	if hasDouyinIssue(m.Errors, DouyinMainImageNotUploaded) || hasDouyinIssue(m.Errors, DouyinMainImageUploadFailed) {
		t.Fatalf("uploaded main image should pass image validation: %+v", m.Errors)
	}
}

func TestDouyinImageDownloadRejectsPrivateNetworkURL(t *testing.T) {
	err := safedownload.ValidateURL(context.Background(), "http://127.0.0.1/image.png")
	if err == nil {
		t.Fatalf("expected private url rejection, got nil")
	}
}

func seedLocalStorage(t *testing.T, db *gorm.DB, root, publicBase string) {
	t.Helper()
	for k, v := range map[string]string{"kind": "local", "local_root": root, "public_base": publicBase} {
		if err := db.Create(&settings.Setting{GroupKey: "storage", ItemKey: k, ItemValue: v, ValueType: "string"}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func validDouyinMapping(shopID string, price float64, stock int) *DouyinDraftMapping {
	return &DouyinDraftMapping{
		Platform:    "douyin_shop",
		ShopID:      shopID,
		CategoryID:  "leaf-1",
		Title:       "Valid Douyin Title",
		Description: "Valid Douyin Description",
		MainImages: []DouyinDraftImage{{
			ImageType:        ImageTypeMain,
			URL:              "https://cdn.example.test/main.png",
			PlatformImageID:  "material-main",
			PlatformImageURL: "https://douyin.example.test/main.png",
			UploadStatus:     DouyinImageStatusUploaded,
		}},
		Attributes: []DouyinDraftAttr{{AttrID: "brand", Name: "Brand", Value: "TradeMind"}},
		SKUs: []DouyinDraftSKU{{
			LocalSkuID: uuid.NewString(),
			Name:       "Default",
			Attrs:      map[string]any{"spec": "default"},
			Price:      price,
			Stock:      &stock,
		}},
		Price: DouyinDraftPrice{Currency: "CNY", Min: &price},
		Stock: DouyinDraftStock{Total: &stock, Min: &stock},
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 80, B: 20, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testGinContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	return c
}

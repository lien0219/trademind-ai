package aiproductimage

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func openImageScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ai_image_scope_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&product.Product{},
		&product.ProductImage{},
		&AIProductImageBatch{},
		&AIProductImageItem{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func imageScopeContext(tenantID int64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func createScopedImageBatch(t *testing.T, db *gorm.DB, tenantID int64) *AIProductImageBatch {
	t.Helper()
	row := &AIProductImageBatch{
		TenantID: tenantID, BatchNo: fmt.Sprintf("SCOPE-%d-%s", tenantID, uuid.NewString()[:8]),
		BatchType: BatchTypeAIImage, Status: BatchFailed, IdempotencyKey: uuid.NewString(),
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func TestTenantScopeListAndGetBatch(t *testing.T) {
	db := openImageScopeTestDB(t)
	svc := &Service{DB: db}
	own := createScopedImageBatch(t, db, 7)
	foreign := createScopedImageBatch(t, db, 8)

	rows, total, err := svc.ListBatches(imageScopeContext(7), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != own.ID.String() {
		t.Fatalf("unexpected tenant-scoped batches: total=%d rows=%+v", total, rows)
	}
	if _, err := svc.GetBatchByID(imageScopeContext(7), foreign.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected foreign batch to be hidden, got %v", err)
	}
}

func TestTenantScopeCheckRejectsForeignProductAndImage(t *testing.T) {
	db := openImageScopeTestDB(t)
	svc := &Service{DB: db}
	productA := &product.Product{TenantID: 7, Source: "test", Title: "A", Status: product.StatusDraft}
	productB := &product.Product{TenantID: 8, Source: "test", Title: "B", Status: product.StatusDraft}
	if err := db.Create(productA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(productB).Error; err != nil {
		t.Fatal(err)
	}
	imageA := &product.ProductImage{ProductID: productA.ID, ImageType: product.ImageTypeMain}
	imageB := &product.ProductImage{ProductID: productB.ID, ImageType: product.ImageTypeMain}
	if err := db.Create(imageA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(imageB).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.CheckBatch(imageScopeContext(7), CheckBatchRequest{
		ProductIDs:     []string{productA.ID.String(), productB.ID.String()},
		ImageIDs:       []string{imageA.ID.String()},
		OperationTypes: []string{OpQualityCheck},
	})
	if err == nil {
		t.Fatal("expected foreign product to be rejected")
	}
	_, err = svc.CheckBatch(imageScopeContext(7), CheckBatchRequest{
		ProductIDs:     []string{productA.ID.String()},
		ImageIDs:       []string{imageB.ID.String()},
		OperationTypes: []string{OpQualityCheck},
	})
	if err == nil {
		t.Fatal("expected foreign image to be rejected")
	}
}

func TestTenantScopeMutationsRejectForeignBatchAndItem(t *testing.T) {
	db := openImageScopeTestDB(t)
	svc := &Service{DB: db}
	foreign := createScopedImageBatch(t, db, 8)
	item := &AIProductImageItem{
		BatchID: foreign.ID, ProductID: uuid.New(), ImageType: product.ImageTypeMain,
		OperationType: OpQualityCheck, Status: ItemPending,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatal(err)
	}
	c := imageScopeContext(7)
	if _, err := svc.CancelPending(c, foreign.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected foreign cancel to be rejected, got %v", err)
	}
	if err := svc.RejectItem(c, item.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected foreign reject to be rejected, got %v", err)
	}
	if _, err := svc.ApplyItem(c, item.ID, ApplyItemBody{ApplyMode: ApplyReplaceImage}, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected foreign apply to be rejected, got %v", err)
	}
	var persisted AIProductImageItem
	if err := db.First(&persisted, "id = ?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Status != ItemPending {
		t.Fatalf("foreign mutation changed item status: %s", persisted.Status)
	}
}

func TestDetachedGinContextPreservesTenantContext(t *testing.T) {
	c := imageScopeContext(7)
	tenant := security.WorkerTenantContext(7, uuid.New())
	security.SetGin(c, tenant)
	c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), tenant))

	detached := detachedGinContext(c)
	got, err := security.RequireTenantContext(detached.Request.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.TenantID != tenant.TenantID || got.UserID != tenant.UserID {
		t.Fatalf("tenant context changed during detach: got=%+v want=%+v", got, tenant)
	}
}

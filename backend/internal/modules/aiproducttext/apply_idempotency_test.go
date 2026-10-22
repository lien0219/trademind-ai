package aiproducttext

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func openTextApplyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ai_text_apply_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
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
		&product.ProductSKU{},
		&product.ProductAIContentApplication{},
		&AIProductTextBatch{},
		&AIProductTextItem{},
		&idempotency.Record{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

var setGinTestMode sync.Once

func testGinContext() *gin.Context {
	setGinTestMode.Do(func() { gin.SetMode(gin.TestMode) })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request = req
	c.Set(ctxkey.TenantID, int64(1))
	c.Set(ctxkey.AdminID, uuid.New().String())
	c.Set("adminperm.principal", &adminperm.Principal{
		Role: adminperm.RoleAdmin, Permissions: adminperm.PermissionsForRole(adminperm.RoleAdmin),
	})
	security.SetGin(c, &security.TenantContext{TenantID: 1, AuthSource: security.AuthSourceAccessToken})
	return c
}

func TestConcurrentApplySameItemOnce(t *testing.T) {
	db := openTextApplyTestDB(t)
	prodSvc := &product.Service{DB: db}
	svc := &Service{
		DB:          db,
		Products:    prodSvc,
		Idempotency: &idempotency.Service{DB: db},
	}

	p := product.Product{
		TenantID: 1,
		Source:   "manual",
		Title:    "Bluetooth earbuds",
		Currency: "USD",
		Status:   product.StatusDraft,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	pu := p.UpdatedAt.UTC()
	batch := AIProductTextBatch{
		BatchNo:      "ATTEST0001",
		BatchType:    BatchTypeAIText,
		Status:       BatchSuccess,
		ProductCount: 1,
		ItemCount:    1,
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	taskID := uuid.New()
	item := AIProductTextItem{
		BatchID:          batch.ID,
		ProductID:        p.ID,
		OperationType:    OpTitle,
		Status:           ItemPendingReview,
		AITaskID:         &taskID,
		GeneratedText:    "AI Optimized Bluetooth Earbuds",
		ProductUpdatedAt: &pu,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}

	const n = 20
	var applied, conflictOrBusy int32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			c := testGinContext()
			c.Set("requestId", fmt.Sprintf("req-%d", idx))
			var loaded AIProductTextItem
			if err := db.First(&loaded, "id = ?", item.ID).Error; err != nil {
				t.Errorf("load item: %v", err)
				return
			}
			r := svc.applyOneItem(c, &loaded, "", nil)
			switch r.Status {
			case ItemApplied:
				atomic.AddInt32(&applied, 1)
			case ItemConflict, ItemProcessing:
				atomic.AddInt32(&conflictOrBusy, 1)
			default:
				t.Errorf("unexpected status=%s err=%s code=%s", r.Status, r.ErrorMessage, r.ErrorCode)
			}
		}(i)
	}
	wg.Wait()

	if applied < 1 {
		t.Fatalf("expected at least one applied, got applied=%d busy=%d", applied, conflictOrBusy)
	}
	var appCount int64
	if err := db.Model(&product.ProductAIContentApplication{}).
		Where("product_id = ? AND status = ?", p.ID, product.AIContentApplyStatusApplied).
		Count(&appCount).Error; err != nil {
		t.Fatal(err)
	}
	if appCount != 1 {
		t.Fatalf("expected exactly 1 application row, got %d (applied responses=%d)", appCount, applied)
	}
	var refreshed product.Product
	if err := db.First(&refreshed, "id = ?", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if refreshed.AITitle != "AI Optimized Bluetooth Earbuds" {
		t.Fatalf("unexpected ai title: %q", refreshed.AITitle)
	}
	var finalItem AIProductTextItem
	if err := db.First(&finalItem, "id = ?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if finalItem.Status != ItemApplied {
		t.Fatalf("concurrent conflict overwrote applied item status: %s", finalItem.Status)
	}
}

func TestApplyTargetVersionConflict(t *testing.T) {
	db := openTextApplyTestDB(t)
	prodSvc := &product.Service{DB: db}
	svc := &Service{
		DB:          db,
		Products:    prodSvc,
		Idempotency: &idempotency.Service{DB: db},
	}
	p := product.Product{Source: "manual", Title: "T", Currency: "USD", Status: product.StatusDraft}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	stale := p.UpdatedAt.Add(-2 * time.Second)
	batch := AIProductTextBatch{BatchNo: "ATTEST0002", BatchType: BatchTypeAIText, Status: BatchSuccess}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	taskID := uuid.New()
	item := AIProductTextItem{
		BatchID: batch.ID, ProductID: p.ID, OperationType: OpTitle,
		Status: ItemPendingReview, AITaskID: &taskID, GeneratedText: "New",
		ProductUpdatedAt: &stale,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	// Bump product updated_at beyond stale snapshot.
	if err := db.Model(&p).Update("title", "T2").Error; err != nil {
		t.Fatal(err)
	}
	r := svc.applyOneItem(testGinContext(), &item, "", nil)
	if r.Status != ItemConflict || r.ErrorCode != ErrCodeTargetVersionConflict {
		t.Fatalf("expected version conflict, got status=%s code=%s msg=%s", r.Status, r.ErrorCode, r.ErrorMessage)
	}
}

func TestApplyAlreadyConflictedItemReturnsConflict(t *testing.T) {
	svc := &Service{}
	item := &AIProductTextItem{Status: ItemConflict}
	result := svc.applyOneItem(testGinContext(), item, "", nil)
	if result.Status != ItemConflict {
		t.Fatalf("expected conflict status, got %s: %s", result.Status, result.ErrorMessage)
	}
}

func TestStaleApplyCannotOverwriteAppliedStatus(t *testing.T) {
	db := openTextApplyTestDB(t)
	p := product.Product{TenantID: 1, Source: "manual", Title: "Original", Currency: "USD", Status: product.StatusDraft}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	batch := AIProductTextBatch{BatchNo: "ATTEST_STALE", BatchType: BatchTypeAIText, Status: BatchSuccess}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	taskID := uuid.New()
	staleVersion := p.UpdatedAt.Add(-2 * time.Second)
	item := AIProductTextItem{
		BatchID: batch.ID, ProductID: p.ID, OperationType: OpTitle,
		Status: ItemApplied, AITaskID: &taskID,
		GeneratedText: "Proposed title", ProductUpdatedAt: &staleVersion,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	staleSnapshot := item
	staleSnapshot.Status = ItemPendingReview
	svc := &Service{DB: db}
	result := svc.applyOneItem(testGinContext(), &staleSnapshot, "", nil)
	if result.Status != ItemConflict {
		t.Fatalf("expected stale apply conflict, got %s: %s", result.Status, result.ErrorMessage)
	}
	var persisted AIProductTextItem
	if err := db.First(&persisted, "id = ?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Status != ItemApplied {
		t.Fatalf("stale attempt overwrote applied item with %s", persisted.Status)
	}
}

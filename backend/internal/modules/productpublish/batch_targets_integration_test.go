package productpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func newBatchIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	safeName := strings.NewReplacer("/", "_", " ", "_", ":", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:batchtest_%s?mode=memory&cache=shared", safeName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
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
		&ProductPublishBatch{},
		&ProductPublishTask{},
		&ProductPublication{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func newBatchTestService(db *gorm.DB) *Service {
	return &Service{
		DB:               db,
		BatchMaxProducts: 100,
		BatchMaxTargets:  20,
		BatchMaxTasks:    300,
	}
}

func testGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	return c
}

func seedBatchProduct(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	adminID := uuid.New()
	pid := uuid.New()
	sid := uuid.New()
	price := 19.9
	stock := 10
	p := product.Product{
		Base:      model.Base{ID: pid},
		Source:    "manual",
		Title:     "Batch Test Product",
		Currency:  "USD",
		Status:    product.StatusDraft,
		CreatedBy: &adminID,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	img := product.ProductImage{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		ProductID:      pid,
		ImageType:      product.ImageTypeMain,
		PublicURL:      "https://example.com/main.jpg",
		SortOrder:      0,
	}
	if err := db.Create(&img).Error; err != nil {
		t.Fatal(err)
	}
	sku := product.ProductSKU{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		ProductID:      pid,
		SKUName:        "Default",
		Price:          &price,
		Stock:          &stock,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}
	return pid, sid
}

func batchCreateReq(pid uuid.UUID, sid uuid.UUID, common map[string]any) BatchTargetsCreateDraftsRequest {
	sidStr := sid.String()
	return BatchTargetsCreateDraftsRequest{
		ProductIDs:      []string{pid.String()},
		Targets:         []PublishTargetRef{{Platform: "shopee", ShopID: &sidStr}},
		CommonConfig:    common,
		IncludeWarnings: true,
	}
}

func TestValidateBatchTaskCountLimit(t *testing.T) {
	svc := newBatchTestService(nil)
	svc.BatchMaxTasks = 300
	if err := svc.validateBatchTaskCount(100, 3); err != nil {
		t.Fatalf("100x3 at limit should pass: %v", err)
	}
	if err := svc.validateBatchTaskCount(101, 3); err == nil {
		t.Fatal("expected limit error for 101x3")
	}
}

func TestCreateBatchIdempotentDuplicate(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	adminID := uuid.New()
	c := testGinContext()
	req := batchCreateReq(pid, sid, map[string]any{"remark": "same"})

	r1, err := svc.CreateBatchTargetDrafts(c, req, &adminID)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.CreateBatchTargetDrafts(c, req, &adminID)
	if err != nil {
		t.Fatal(err)
	}
	if r1.BatchID != r2.BatchID {
		t.Fatalf("expected same batch, got %s vs %s", r1.BatchID, r2.BatchID)
	}
	var batchCount int64
	db.Model(&ProductPublishBatch{}).Count(&batchCount)
	if batchCount != 1 {
		t.Fatalf("expected 1 batch row, got %d", batchCount)
	}
}

func TestCreateBatchDoubleSubmitSameTasks(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	adminID := uuid.New()
	c := testGinContext()
	req := batchCreateReq(pid, sid, nil)

	if _, err := svc.CreateBatchTargetDrafts(c, req, &adminID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateBatchTargetDrafts(c, req, &adminID); err != nil {
		t.Fatal(err)
	}
	var taskCount int64
	db.Model(&ProductPublishTask{}).Count(&taskCount)
	if taskCount != 1 {
		t.Fatalf("expected 1 task after double submit, got %d", taskCount)
	}
}

func TestRetryFailedDoesNotRetrySuccess(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	adminID := uuid.New()
	batchID := uuid.New()
	inRaw, _ := json.Marshal(batchCreateReq(pid, sid, nil))
	batch := ProductPublishBatch{
		HardDeleteBase: model.HardDeleteBase{ID: batchID},
		BatchType:      BatchTypeMultiProduct,
		Status:         BatchPartialSuccess,
		ProductCount:   1,
		TargetCount:    1,
		TaskCount:      1,
		Input:          datatypes.JSON(inRaw),
		CreatedBy:      &adminID,
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	successTask := ProductPublishTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		ProductID:      pid,
		ShopID:         sid,
		TargetStoreID:  sid,
		BatchID:        &batchID,
		TargetKey:      publishTargetKey("shopee", &sid),
		Platform:       "shopee",
		TaskType:       TaskTypeLocalDraftCreate,
		Status:         TaskSuccess,
		Mode:           PublishModeSaveAsPlatformDraft,
		PublishMode:    PublishModeSaveAsPlatformDraft,
	}
	failedTask := successTask
	failedTask.ID = uuid.New()
	failedTask.Status = TaskFailed
	failedTask.ErrorMessage = "boom"
	if err := db.Create(&successTask).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&failedTask).Error; err != nil {
		t.Fatal(err)
	}

	c := testGinContext()
	res, err := svc.RetryFailedBatchTasks(c, batchID, &adminID)
	if err != nil {
		t.Fatal(err)
	}
	if res.SuccessCount < 1 {
		t.Fatalf("expected success preserved, got %+v", res)
	}
	var successRows int64
	db.Model(&ProductPublishTask{}).Where("batch_id = ? AND status = ?", batchID, TaskSuccess).Count(&successRows)
	if successRows < 1 {
		t.Fatal("success task should remain")
	}
}

func TestRetryConcurrentSingleClaim(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	adminID := uuid.New()
	batchID := uuid.New()
	inRaw, _ := json.Marshal(batchCreateReq(pid, sid, nil))
	batch := ProductPublishBatch{
		HardDeleteBase: model.HardDeleteBase{ID: batchID},
		BatchType:      BatchTypeMultiProduct,
		Status:         BatchFailed,
		ProductCount:   1,
		TargetCount:    1,
		TaskCount:      1,
		Input:          datatypes.JSON(inRaw),
		CreatedBy:      &adminID,
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	ft := ProductPublishTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		ProductID:      pid,
		ShopID:         sid,
		TargetStoreID:  sid,
		BatchID:        &batchID,
		TargetKey:      publishTargetKey("shopee", &sid),
		Platform:       "shopee",
		TaskType:       TaskTypeLocalDraftCreate,
		Status:         TaskFailed,
		Mode:           PublishModeSaveAsPlatformDraft,
		PublishMode:    PublishModeSaveAsPlatformDraft,
		ErrorMessage:   "fail",
	}
	if err := db.Create(&ft).Error; err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	var okCount int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := testGinContext()
			if _, err := svc.RetryFailedBatchTasks(c, batchID, &adminID); err == nil {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	if okCount != 2 {
		t.Fatalf("both retries should return ok, got %d", okCount)
	}
	var newSuccess int64
	db.Model(&ProductPublishTask{}).Where("batch_id = ? AND status = ? AND id <> ?", batchID, TaskSuccess, ft.ID).Count(&newSuccess)
	if newSuccess != 1 {
		t.Fatalf("expected exactly 1 new success task from concurrent retry, got %d", newSuccess)
	}
}

func TestCancelPendingPreservesSuccessAndRunning(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	adminID := uuid.New()
	batchID := uuid.New()
	batch := ProductPublishBatch{
		HardDeleteBase: model.HardDeleteBase{ID: batchID},
		BatchType:      BatchTypeMultiProduct,
		Status:         BatchRunning,
		ProductCount:   1,
		TargetCount:    3,
		TaskCount:      3,
		CreatedBy:      &adminID,
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	mk := func(status string) ProductPublishTask {
		return ProductPublishTask{
			HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
			ProductID:      pid,
			ShopID:         sid,
			TargetStoreID:  sid,
			BatchID:        &batchID,
			TargetKey:      publishTargetKey("shopee", &sid),
			Platform:       "shopee",
			TaskType:       TaskTypeLocalDraftCreate,
			Status:         status,
			Mode:           PublishModeSaveAsPlatformDraft,
			PublishMode:    PublishModeSaveAsPlatformDraft,
		}
	}
	for _, st := range []string{TaskPending, TaskRunning, TaskSuccess} {
		row := mk(st)
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	c := testGinContext()
	out, err := svc.CancelPendingBatchTasks(c, batchID, &adminID)
	if err != nil {
		t.Fatal(err)
	}
	var pending, running, success int64
	db.Model(&ProductPublishTask{}).Where("batch_id = ? AND status = ?", batchID, TaskPending).Count(&pending)
	db.Model(&ProductPublishTask{}).Where("batch_id = ? AND status = ?", batchID, TaskRunning).Count(&running)
	db.Model(&ProductPublishTask{}).Where("batch_id = ? AND status = ?", batchID, TaskSuccess).Count(&success)
	if pending != 0 || running != 1 || success != 1 {
		t.Fatalf("pending=%d running=%d success=%d", pending, running, success)
	}
	if out == nil {
		t.Fatal("nil detail")
	}
}

func TestDouyinSuccessDedupSkipsCreate(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	eff := mergeEffectiveConfig(nil, PublishConfigOverrides{}, pid.String(), "douyin_shop", sid.String())
	keyRaw, _ := json.Marshal(map[string]any{"idempotencyKey": taskIdempotencyKey(pid.String(), "douyin_shop", sid.String(), eff)})
	existing := ProductPublishTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		ProductID:      pid,
		ShopID:         sid,
		TargetStoreID:  sid,
		TargetKey:      publishTargetKey("douyin_shop", &sid),
		Platform:       "douyin_shop",
		TaskType:       TaskTypeDouyinDraftCreate,
		Status:         TaskSuccess,
		Mode:           PublishModeSaveAsPlatformDraft,
		PublishMode:    PublishModeSaveAsPlatformDraft,
		Input:          datatypes.JSON(keyRaw),
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	dup, ok := svc.findExistingSuccessfulTask(context.Background(), pid, "douyin_shop", &sid, eff)
	if !ok || dup == nil {
		t.Fatal("expected douyin dedup")
	}
}

func TestLocalDraftDedupSkipsPublication(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	adminID := uuid.New()
	c := testGinContext()
	req := batchCreateReq(pid, sid, map[string]any{"priceRule": "fixed"})
	if _, err := svc.CreateBatchTargetDrafts(c, req, &adminID); err != nil {
		t.Fatal(err)
	}
	var pubCount1 int64
	db.Model(&ProductPublication{}).Count(&pubCount1)
	if pubCount1 != 1 {
		t.Fatalf("expected 1 publication, got %d", pubCount1)
	}
	if _, err := svc.CreateBatchTargetDrafts(c, req, &adminID); err != nil {
		t.Fatal(err)
	}
	var pubCount2 int64
	db.Model(&ProductPublication{}).Count(&pubCount2)
	if pubCount2 != 1 {
		t.Fatalf("expected still 1 publication after idempotent batch, got %d", pubCount2)
	}
}

func TestDifferentConfigHashAllowsNewTask(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	adminID := uuid.New()
	c := testGinContext()

	req1 := batchCreateReq(pid, sid, map[string]any{"priceRule": "a"})
	res1, err := svc.CreateBatchTargetDrafts(c, req1, &adminID)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Model(&ProductPublishBatch{}).Where("id = ?", res1.BatchID).Updates(map[string]any{
		"status": BatchFailed, "idempotency_key": "",
	}).Error

	req2 := batchCreateReq(pid, sid, map[string]any{"priceRule": "b"})
	res2, err := svc.CreateBatchTargetDrafts(c, req2, &adminID)
	if err != nil {
		t.Fatal(err)
	}
	if res1.BatchID == res2.BatchID {
		t.Fatal("different config should create new batch")
	}
}

func TestBatchAccessDeniedForOtherAdmin(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	owner := uuid.New()
	other := uuid.New()
	batchID := uuid.New()
	batch := ProductPublishBatch{
		HardDeleteBase: model.HardDeleteBase{ID: batchID, CreatedAt: time.Now().UTC()},
		BatchType:      BatchTypeMultiProduct,
		Status:         BatchSuccess,
		CreatedBy:      &owner,
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.GetPublishBatchDetail(context.Background(), batchID, &other)
	if err != ErrBatchAccessDenied {
		t.Fatalf("expected ErrBatchAccessDenied, got %v", err)
	}
}

func TestSameConfigHashSkipsTaskCreate(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	eff := mergeEffectiveConfig(map[string]any{"remark": "x"}, PublishConfigOverrides{}, pid.String(), "shopee", sid.String())
	wantKey := taskIdempotencyKey(pid.String(), "shopee", sid.String(), eff)
	keyRaw, _ := json.Marshal(map[string]any{"idempotencyKey": wantKey})
	row := ProductPublishTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		ProductID:      pid,
		ShopID:         sid,
		TargetStoreID:  sid,
		TargetKey:      publishTargetKey("shopee", &sid),
		Platform:       "shopee",
		TaskType:       TaskTypeLocalDraftCreate,
		Status:         TaskSuccess,
		Mode:           PublishModeSaveAsPlatformDraft,
		PublishMode:    PublishModeSaveAsPlatformDraft,
		Input:          datatypes.JSON(keyRaw),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	dup, ok := svc.findExistingSuccessfulTask(context.Background(), pid, "shopee", &sid, eff)
	if !ok || dup.ID != row.ID {
		t.Fatal("expected same-config dedup")
	}
	otherEff := mergeEffectiveConfig(map[string]any{"remark": "y"}, PublishConfigOverrides{}, pid.String(), "shopee", sid.String())
	if _, ok := svc.findExistingSuccessfulTask(context.Background(), pid, "shopee", &sid, otherEff); ok {
		t.Fatal("different config should not dedup")
	}
}

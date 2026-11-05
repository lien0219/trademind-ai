package aioperationbatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestAIBatchRoutesEnforcePermissionAndTenantScope(t *testing.T) {
	db, actor, role := newAIBatchAuthorizationFixture(t)
	readRole := admin.RoleDefinition{TenantID: actor.TenantID, Code: role, Name: "Batch reader", PermissionsJSON: `[]`, Enabled: true}
	encoded, err := json.Marshal([]string{adminperm.PermProductView})
	require.NoError(t, err)
	readRole.PermissionsJSON = string(encoded)
	require.NoError(t, db.Create(&readRole).Error)
	storeID := uuid.New()
	require.NoError(t, db.Create(&shop.Shop{Base: model.Base{ID: storeID}, TenantID: actor.TenantID, Platform: "manual", ShopName: "A", Status: "active", AuthStatus: "authorized"}).Error)
	require.NoError(t, db.Create(&admin.UserStorePermission{UserID: actor.ID, StoreID: storeID, PermissionScope: admin.StorePermScopeView}).Error)
	productRow := product.Product{TenantID: actor.TenantID, Source: "manual", Title: "authorized"}
	require.NoError(t, db.Create(&productRow).Error)
	require.NoError(t, db.Create(&product.ProductPlatformPublishConfig{ProductID: productRow.ID, Platform: "manual", ShopID: &storeID}).Error)

	first := AIOperationBatch{TenantID: actor.TenantID, BatchNo: "tenant-a", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	foreign := AIOperationBatch{TenantID: actor.TenantID + 1, BatchNo: "tenant-b", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&foreign).Error)
	require.NoError(t, db.Create(&aitask.AITask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TaskType:       "title_optimize", Status: aitask.StatusSuccess,
		ProductID: &productRow.ID, BatchID: &first.ID,
	}).Error)

	router := newAIBatchAuthorizationRouter(db, actor, role)
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/ai/batches", nil))
	require.Equal(t, http.StatusOK, list.Code)
	var payload struct {
		Data struct {
			List []AIOperationBatch `json:"list"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &payload))
	require.Len(t, payload.Data.List, 1)
	require.Equal(t, first.ID, payload.Data.List[0].ID)
	for _, suffix := range []string{"", "/tasks"} {
		own := httptest.NewRecorder()
		router.ServeHTTP(own, httptest.NewRequest(http.MethodGet, "/api/v1/ai/batches/"+first.ID.String()+suffix, nil))
		require.Equal(t, http.StatusOK, own.Code, own.Body.String())
	}

	detail := httptest.NewRecorder()
	router.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/v1/ai/batches/"+foreign.ID.String(), nil))
	require.Equal(t, http.StatusNotFound, detail.Code)

	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ai/batches/product-text", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(create, request)
	require.Equal(t, http.StatusForbidden, create.Code)
}

func TestAIBatchCreateRollsBackWhenAuditCannotBePersisted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai_batch_audit_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AIOperationBatch{}, &operationlog.OperationLog{}))
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_ai_batch_audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("ai batch audit unavailable"))
		}
	}))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/ai/batches/product-text", nil)
	c.Set(ctxkey.TenantID, int64(41))
	c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), &security.TenantContext{TenantID: 41}))
	svc := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
	batch := &AIOperationBatch{TenantID: 41, BatchNo: "audit-rollback", OperationType: OperationTitleOptimize, Status: StatusRunning}
	err = svc.createBatchWithAudit(c, batch, operationlog.WriteOpts{
		TenantID: 41, Action: "ai.batch.create", Resource: "ai_operation_batch", ResourceID: batch.ID.String(), Status: "success",
	})
	require.ErrorContains(t, err, "ai batch audit unavailable")
	var count int64
	require.NoError(t, db.Model(&AIOperationBatch{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestAIBatchApplyRollsBackProductChangeWhenAuditCannotBePersisted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai_batch_apply_audit_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&product.Product{}, &AIOperationBatch{}, &aitask.AITask{}, &operationlog.OperationLog{}))

	productID := uuid.New()
	batchID := uuid.New()
	prod := product.Product{
		Base:     model.Base{ID: productID},
		TenantID: 41,
		Source:   "manual",
		Title:    "Original",
		AITitle:  "old AI title",
		Status:   product.StatusDraft,
	}
	batch := AIOperationBatch{
		Base:          model.Base{ID: batchID},
		TenantID:      41,
		BatchNo:       "apply-audit-rollback",
		OperationType: OperationTitleOptimize,
		Status:        StatusSuccess,
	}
	task := aitask.AITask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TaskType:       "title_optimize",
		Status:         aitask.StatusSuccess,
		ProductID:      &productID,
		BatchID:        &batchID,
		Output:         datatypes.JSON(`{"optimizedTitle":"new AI title"}`),
	}
	require.NoError(t, db.Create(&prod).Error)
	require.NoError(t, db.Create(&batch).Error)
	require.NoError(t, db.Create(&task).Error)
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_ai_batch_apply_audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("ai batch apply audit unavailable"))
		}
	}))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/ai/batches/"+batchID.String()+"/apply-results", nil)
	c.Set(ctxkey.TenantID, int64(41))
	c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), &security.TenantContext{TenantID: 41}))
	svc := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}

	_, err = svc.ApplyBatchResults(c, batchID, ApplyBatchResultsBody{Target: applyTargetAIField}, nil)
	require.ErrorContains(t, err, "ai batch apply audit unavailable")

	var stored product.Product
	require.NoError(t, db.First(&stored, "id = ?", productID).Error)
	require.Equal(t, "old AI title", stored.AITitle, "product writes must roll back with the failed audit")
	var auditCount int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Count(&auditCount).Error)
	require.Zero(t, auditCount)
}

func TestCreateImageTaskWithAuditRollsBackWhenAuditCannotBePersisted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai_image_task_audit_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&imagetask.ImageTask{}, &operationlog.OperationLog{}))
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_ai_image_task_audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("ai image task audit unavailable"))
		}
	}))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/ai/batches/product-images", nil)
	c.Set(ctxkey.TenantID, int64(41))
	c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), &security.TenantContext{TenantID: 41}))
	svc := &Service{
		DB:    db,
		Image: &imagetask.Service{DB: db},
		OpLog: &operationlog.Service{DB: db},
	}

	_, err = svc.createImageTaskWithAudit(c, imagetask.CreatePayload{
		TaskType:       imagetask.TaskTypeGenerateScene,
		Provider:       "local_render",
		SourceImageURL: "https://example.test/source.jpg",
		Input:          datatypes.JSON(`{}`),
	}, operationlog.WriteOpts{
		TenantID: 41, Action: "image.task.create", Resource: "image_task", Status: "success",
	})
	require.ErrorContains(t, err, "ai image task audit unavailable")
	var count int64
	require.NoError(t, db.Model(&imagetask.ImageTask{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestReconcileImageBatchDoesNotMixTenants(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai_batch_reconcile_scope_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AIOperationBatch{}, &imagetask.ImageTask{}))

	batchID := uuid.New()
	require.NoError(t, db.Create(&AIOperationBatch{
		Base:          model.Base{ID: batchID},
		TenantID:      41,
		BatchNo:       "scope-check",
		OperationType: OperationImageGenerateScene,
		Status:        StatusRunning,
	}).Error)
	require.NoError(t, db.Create(&imagetask.ImageTask{
		TenantID: 41, BatchID: &batchID, TaskType: imagetask.TaskTypeGenerateScene,
		Provider: "local", Status: imagetask.StatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&imagetask.ImageTask{
		TenantID: 42, BatchID: &batchID, TaskType: imagetask.TaskTypeGenerateScene,
		Provider: "local", Status: imagetask.StatusFailed,
	}).Error)

	ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 41})
	svc := &Service{DB: db}
	require.NoError(t, svc.reconcileImageBatch(ctx, batchID))

	var got AIOperationBatch
	require.NoError(t, db.First(&got, "id = ?", batchID).Error)
	require.Equal(t, StatusSuccess, got.Status)
	require.Equal(t, 1, got.TaskCount)
	require.Equal(t, 1, got.SuccessCount)
	require.Equal(t, 0, got.FailedCount)
}

func TestRetryFailedPermissionDeniedDoesNotReconcileImageBatch(t *testing.T) {
	db, actor, role := newAIBatchAuthorizationFixture(t)
	readRole := admin.RoleDefinition{TenantID: actor.TenantID, Code: role, Name: "Batch reader", PermissionsJSON: `[]`, Enabled: true}
	encoded, err := json.Marshal([]string{adminperm.PermProductView})
	require.NoError(t, err)
	readRole.PermissionsJSON = string(encoded)
	require.NoError(t, db.Create(&readRole).Error)
	storeID := uuid.New()
	require.NoError(t, db.Create(&shop.Shop{Base: model.Base{ID: storeID}, TenantID: actor.TenantID, Platform: "manual", ShopName: "A", Status: "active", AuthStatus: "authorized"}).Error)
	require.NoError(t, db.Create(&admin.UserStorePermission{UserID: actor.ID, StoreID: storeID, PermissionScope: admin.StorePermScopeView}).Error)
	productRow := product.Product{TenantID: actor.TenantID, Source: "manual", Title: "authorized"}
	require.NoError(t, db.Create(&productRow).Error)
	require.NoError(t, db.Create(&product.ProductPlatformPublishConfig{ProductID: productRow.ID, Platform: "manual", ShopID: &storeID}).Error)

	batchID := uuid.New()
	require.NoError(t, db.Create(&AIOperationBatch{
		Base:          model.Base{ID: batchID},
		TenantID:      actor.TenantID,
		BatchNo:       "retry-permission-order",
		OperationType: OperationImageGenerateScene,
		Status:        StatusRunning,
	}).Error)
	require.NoError(t, db.Create(&imagetask.ImageTask{
		TenantID:  actor.TenantID,
		BatchID:   &batchID,
		TaskType:  imagetask.TaskTypeGenerateScene,
		Provider:  "local",
		Status:    imagetask.StatusFailed,
		ProductID: &productRow.ID,
	}).Error)

	router := newAIBatchAuthorizationRouter(db, actor, role)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/ai/batches/"+batchID.String()+"/retry-failed", nil))
	require.Equal(t, http.StatusForbidden, rec.Code)

	var got AIOperationBatch
	require.NoError(t, db.First(&got, "id = ?", batchID).Error)
	require.Equal(t, StatusRunning, got.Status, "permission denial must happen before image reconciliation")
	require.Zero(t, got.TaskCount)
}

func TestAITaskBatchCountsRespectTenantScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai_batch_task_count_scope_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AIOperationBatch{}, &aitask.AITask{}))

	batchA := uuid.New()
	batchB := uuid.New()
	require.NoError(t, db.Create(&AIOperationBatch{
		Base:          model.Base{ID: batchA},
		TenantID:      41,
		BatchNo:       "count-tenant-a",
		OperationType: OperationTitleOptimize,
		Status:        StatusRunning,
	}).Error)
	require.NoError(t, db.Create(&AIOperationBatch{
		Base:          model.Base{ID: batchB},
		TenantID:      42,
		BatchNo:       "count-tenant-b",
		OperationType: OperationTitleOptimize,
		Status:        StatusRunning,
	}).Error)
	productA := uuid.New()
	productB := uuid.New()
	require.NoError(t, db.Create(&aitask.AITask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TaskType:       "title_optimize",
		Status:         aitask.StatusSuccess,
		ProductID:      &productA,
		BatchID:        &batchA,
	}).Error)
	require.NoError(t, db.Create(&aitask.AITask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TaskType:       "title_optimize",
		Status:         aitask.StatusFailed,
		ProductID:      &productB,
		BatchID:        &batchB,
	}).Error)

	svc := &Service{DB: db}
	ctxA := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 41})
	n, err := svc.countAITasksForBatch(ctxA, batchA)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	success, failed, err := svc.countLatestAITasksByProductStatus(ctxA, batchA)
	require.NoError(t, err)
	require.Equal(t, 1, success)
	require.Zero(t, failed)

	foreignCount, err := svc.countAITasksForBatch(ctxA, batchB)
	require.NoError(t, err)
	require.Zero(t, foreignCount)
	foreignSuccess, foreignFailed, err := svc.countLatestAITasksByProductStatus(ctxA, batchB)
	require.NoError(t, err)
	require.Zero(t, foreignSuccess)
	require.Zero(t, foreignFailed)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/ai/batches/"+batchA.String()+"/tasks", nil).WithContext(ctxA)
	rows, total, err := svc.ListBatchAITasks(c, batchA, 1, 10)
	require.NoError(t, err, "an authorized batch must return its paginated task rows")
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, productA.String(), rows[0]["productId"])
	rows, total, err = svc.ListBatchAITasks(c, batchA, 2, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Empty(t, rows)
	rows, total, err = svc.ListBatchAITasks(c, batchB, 1, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, rows)
}

func newAIBatchAuthorizationFixture(t *testing.T) (*gorm.DB, admin.AdminUser, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ai_batch_authz_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}, &product.Product{}, &product.ProductPlatformPublishConfig{}, &productpublish.ProductPublication{}, &AIOperationBatch{}, &aitask.AITask{}, &imagetask.ImageTask{}, &operationlog.OperationLog{}))
	actor := admin.AdminUser{TenantID: 41, Username: admin.NewInternalUsername(), Email: "batch-reader@example.test", PasswordHash: "test-only", Role: "r_0123456789abcdef01234567", Status: admin.StatusActive, TokenVersion: 1}
	require.NoError(t, db.Create(&actor).Error)
	return db, actor, actor.Role
}

func newAIBatchAuthorizationRouter(db *gorm.DB, actor admin.AdminUser, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Set(ctxkey.TenantID, actor.TenantID)
		tenant := security.BuildTenantContext(c, actor.TenantID, actor.ID, uuid.Nil, role, nil, nil)
		security.SetGin(c, tenant)
		c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), tenant))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db, OpLog: &operationlog.Service{DB: db}}})
	return router
}

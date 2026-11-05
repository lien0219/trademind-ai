package imagetask

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func openImageTenantScopeDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:image_task_tenant_scope_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&files.FileRecord{}, &product.Product{}, &product.ProductImage{}, &ImageTask{}, &ImageTaskItem{}))
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func imageTenantContext(tenantID int64) context.Context {
	return security.WithTenantContext(context.Background(), &security.TenantContext{
		TenantID:   tenantID,
		AuthSource: security.AuthSourceAccessToken,
	})
}

func TestImageTaskServiceScopesTasksItemsSourcesAndApplyByTenant(t *testing.T) {
	db := openImageTenantScopeDB(t)
	svc := &Service{DB: db}

	productA := product.Product{TenantID: 41, Source: "manual", Title: "A", Status: product.StatusDraft}
	productB := product.Product{TenantID: 42, Source: "manual", Title: "B", Status: product.StatusDraft}
	require.NoError(t, db.Create(&productA).Error)
	require.NoError(t, db.Create(&productB).Error)

	imageB := product.ProductImage{ProductID: productB.ID, ImageType: product.ImageTypeMain, PublicURL: "https://tenant-b.example/image.jpg"}
	require.NoError(t, db.Create(&imageB).Error)
	taskA := ImageTask{TenantID: 41, TaskType: TaskTypeGenerateScene, Provider: "test", Status: StatusSuccess, ProductID: &productA.ID, ResultURL: "https://tenant-a.example/result.jpg"}
	taskB := ImageTask{TenantID: 42, TaskType: TaskTypeGenerateScene, Provider: "test", Status: StatusSuccess, ProductID: &productB.ID, SourceImageID: &imageB.ID, ResultURL: "https://tenant-b.example/result.jpg"}
	require.NoError(t, db.Create(&taskA).Error)
	require.NoError(t, db.Create(&taskB).Error)
	itemA := ImageTaskItem{TaskID: taskA.ID, ProductID: &productA.ID, Status: ItemStatusSuccess, OutputImageURL: "https://tenant-a.example/item.jpg"}
	itemB := ImageTaskItem{TaskID: taskB.ID, ProductID: &productB.ID, Status: ItemStatusSuccess, OutputImageURL: "https://tenant-b.example/item.jpg"}
	require.NoError(t, db.Create(&itemA).Error)
	require.NoError(t, db.Create(&itemB).Error)

	ctxA := imageTenantContext(41)
	got, err := svc.GetByID(ctxA, taskA.ID)
	require.NoError(t, err)
	require.Equal(t, taskA.ID, got.ID)
	_, err = svc.GetByID(ctxA, taskB.ID)
	require.Error(t, err)

	items, err := svc.ListTaskItems(ctxA, taskA.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	_, err = svc.ListTaskItems(ctxA, taskB.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = svc.GetTaskItem(ctxA, itemB.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.ErrorIs(t, svc.DeleteTaskItem(ctxA, taskB.ID, itemB.ID), gorm.ErrRecordNotFound)
	var itemStillThere ImageTaskItem
	require.NoError(t, db.First(&itemStillThere, "id = ?", itemB.ID).Error)

	_, _, err = svc.ResolveSource(ctxA, &imageB.ID, "")
	require.Error(t, err)
	_, err = svc.ApplyTaskResult(ctxA, ApplyItemOpts{TaskID: taskB.ID, ProductID: productB.ID})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = svc.CreateSelectBestMainTask(ctxA, productB.ID, "recommend", nil)
	require.Error(t, err)
	require.NotErrorIs(t, err, security.ErrTenantContextMissing)
}

func TestImageTaskMonitorScopesCountsByTenant(t *testing.T) {
	db := openImageTenantScopeDB(t)
	svc := &Service{DB: db}
	for _, task := range []ImageTask{
		{TenantID: 41, TaskType: TaskTypeGenerateScene, Provider: "test", Status: StatusFailed},
		{TenantID: 42, TaskType: TaskTypeGenerateScene, Provider: "test", Status: StatusFailed},
		{TenantID: 42, TaskType: TaskTypeGenerateScene, Provider: "test", Status: StatusSuccess},
	} {
		require.NoError(t, db.Create(&task).Error)
	}
	counts, err := svc.CountTasksByStatus(imageTenantContext(41))
	require.NoError(t, err)
	require.Equal(t, int64(1), counts.Failed)
	require.Zero(t, counts.Success)
}

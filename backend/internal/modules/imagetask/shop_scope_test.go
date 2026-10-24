package imagetask

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func TestImageTaskShopScopeFiltersTasksSourcesAndCreate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image_task_shop_scope_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&shop.Shop{},
		&product.Product{},
		&product.ProductPlatformPublishConfig{},
		&productpublish.ProductPublication{},
		&product.ProductImage{},
		&files.FileRecord{},
		&ImageTask{},
		&ImageTaskItem{},
	))

	shopA := uuid.New()
	shopB := uuid.New()
	require.NoError(t, db.Create(&shop.Shop{Base: model.Base{ID: shopA}, TenantID: 41, Platform: "manual", ShopName: "A", Status: "active", AuthStatus: "authorized"}).Error)
	require.NoError(t, db.Create(&shop.Shop{Base: model.Base{ID: shopB}, TenantID: 41, Platform: "manual", ShopName: "B", Status: "active", AuthStatus: "authorized"}).Error)
	productA := product.Product{TenantID: 41, Source: "manual", Title: "A"}
	productB := product.Product{TenantID: 41, Source: "manual", Title: "B"}
	require.NoError(t, db.Create(&productA).Error)
	require.NoError(t, db.Create(&productB).Error)
	require.NoError(t, db.Create(&product.ProductPlatformPublishConfig{ProductID: productA.ID, Platform: "manual", ShopID: &shopA}).Error)
	require.NoError(t, db.Create(&product.ProductPlatformPublishConfig{ProductID: productB.ID, Platform: "manual", ShopID: &shopB}).Error)
	imageB := product.ProductImage{ProductID: productB.ID, ImageType: product.ImageTypeMain, PublicURL: "https://tenant-b.example/source.jpg"}
	require.NoError(t, db.Create(&imageB).Error)
	taskA := ImageTask{TenantID: 41, TaskType: TaskTypeGenerateScene, Provider: "local_render", Status: StatusSuccess, ProductID: &productA.ID}
	taskB := ImageTask{TenantID: 41, TaskType: TaskTypeGenerateScene, Provider: "local_render", Status: StatusFailed, ProductID: &productB.ID, SourceImageID: &imageB.ID}
	require.NoError(t, db.Create(&taskA).Error)
	require.NoError(t, db.Create(&taskB).Error)

	ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 41, ShopScope: []uuid.UUID{shopA}})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/image/tasks", nil).WithContext(ctx)
	svc := &Service{DB: db}
	res, err := svc.List(c, ListQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, res.Total)
	require.Len(t, res.Items, 1)
	require.Equal(t, taskA.ID, res.Items[0].ID)
	_, err = svc.GetByID(ctx, taskB.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, _, err = svc.ResolveSource(ctx, &imageB.ID, "")
	require.Error(t, err)
	fileB := files.FileRecord{TenantID: 41, OriginalName: "b.jpg", ObjectKey: "tenant-b/b.jpg", PublicURL: "https://tenant-b.example/file.jpg", StorageKind: "local"}
	require.NoError(t, db.Create(&fileB).Error)
	_, _, err = svc.ResolveSource(ctx, &fileB.ID, "")
	require.Error(t, err, "unscoped file records cannot prove shop ownership")

	_, err = svc.CreateAndPersist(ctx, CreatePayload{
		TaskType: TaskTypeGenerateScene, Provider: "local_render", ProductID: &productB.ID,
		Input: []byte(`{}`),
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&ImageTask{}).Where("product_id = ?", productB.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

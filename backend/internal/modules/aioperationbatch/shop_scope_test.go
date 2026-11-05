package aioperationbatch

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func TestAIBatchShopScopeFiltersBatchesProductsAndImageTasks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai_batch_shop_scope_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&shop.Shop{},
		&product.Product{},
		&product.ProductPlatformPublishConfig{},
		&productpublish.ProductPublication{},
		&AIOperationBatch{},
		&aitask.AITask{},
		&imagetask.ImageTask{},
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

	batchA := AIOperationBatch{TenantID: 41, BatchNo: "scope-a", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	batchB := AIOperationBatch{TenantID: 41, BatchNo: "scope-b", OperationType: OperationImageGenerateScene, Status: StatusSuccess}
	require.NoError(t, db.Create(&batchA).Error)
	require.NoError(t, db.Create(&batchB).Error)
	require.NoError(t, db.Create(&aitask.AITask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, TaskType: "title_optimize", Status: aitask.StatusSuccess,
		ProductID: &productA.ID, BatchID: &batchA.ID,
	}).Error)
	require.NoError(t, db.Create(&imagetask.ImageTask{
		TenantID: 41, BatchID: &batchB.ID, TaskType: imagetask.TaskTypeGenerateScene,
		Provider: "local", Status: imagetask.StatusSuccess, ProductID: &productB.ID,
	}).Error)

	ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 41, ShopScope: []uuid.UUID{shopA}})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/ai/batches", nil).WithContext(ctx)
	svc := &Service{DB: db, Image: &imagetask.Service{DB: db}}

	items, total, err := svc.ListBatches(c, ListBatchesQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, batchA.ID, items[0].ID)
	_, err = svc.GetByID(ctx, batchB.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	aiRows, aiTotal, err := svc.ListBatchAITasks(c, batchA.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, aiTotal)
	require.Len(t, aiRows, 1)
	imageRows, imageTotal, err := svc.ListBatchImageTasks(c, batchB.ID, 1, 20)
	require.NoError(t, err)
	require.Zero(t, imageTotal)
	require.Empty(t, imageRows)

	_, err = svc.resolveProductIDs(ctx, 41, []uuid.UUID{productB.ID}, ProductFilters{}, 20, false)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	ids, err := svc.resolveProductIDs(ctx, 41, nil, ProductFilters{}, 20, true)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{productA.ID}, ids)
}

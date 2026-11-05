package imagetask

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

type retrySchedulerRedisHook struct {
	pushes int
}

func TestImageRetrySchedulerDoesNotEnqueueForCrossTenantShopLink(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image_retry_shop_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &shop.Shop{}, &product.Product{}, &product.ProductPlatformPublishConfig{}, &productpublish.ProductPublication{}, &ImageTask{}))
	tenant := admin.Tenant{Code: "image-shop-owner", Name: "Image shop owner", Status: admin.TenantActive, Version: 1}
	foreignTenant := admin.Tenant{Code: "image-shop-foreign", Name: "Image shop foreign", Status: admin.TenantActive, Version: 1}
	require.NoError(t, db.Create(&tenant).Error)
	require.NoError(t, db.Create(&foreignTenant).Error)
	foreignShop := shop.Shop{TenantID: foreignTenant.ID, Platform: "mock", ShopName: "foreign", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, db.Create(&foreignShop).Error)
	productRow := product.Product{TenantID: tenant.ID, Source: "manual", Title: "owned", Status: product.StatusDraft}
	require.NoError(t, db.Create(&productRow).Error)
	require.NoError(t, db.Create(&product.ProductPlatformPublishConfig{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		ProductID:      productRow.ID,
		Platform:       "mock",
		ShopID:         &foreignShop.ID,
	}).Error)
	nextRetry := time.Now().UTC().Add(-time.Minute)
	task := ImageTask{
		TenantID: tenant.ID, ProductID: &productRow.ID, TaskType: TaskTypeGenerateScene, Provider: "test",
		Status: StatusRetrying, NextRetryAt: &nextRetry,
	}
	require.NoError(t, db.Create(&task).Error)

	hook := &retrySchedulerRedisHook{}
	queue := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	queue.AddHook(hook)
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	svc := &Service{DB: db, Redis: &rdb.Client{Client: queue}, QueueEnabled: true, AutoRetryEnabled: true}

	svc.flushImageDueRetries(context.Background(), nil)

	require.Zero(t, hook.pushes)
	var stored ImageTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusRetrying, stored.Status)
	require.NotNil(t, stored.NextRetryAt)
	require.Nil(t, stored.RetryEnqueuedAt)
}

func TestImageRetrySchedulerDoesNotEnqueueForCrossTenantPublicationShopLink(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image_retry_publication_shop_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &shop.Shop{}, &product.Product{}, &productpublish.ProductPublication{}, &ImageTask{}))
	tenant := admin.Tenant{Code: "image-publication-owner", Name: "Image publication owner", Status: admin.TenantActive, Version: 1}
	foreignTenant := admin.Tenant{Code: "image-publication-foreign", Name: "Image publication foreign", Status: admin.TenantActive, Version: 1}
	require.NoError(t, db.Create(&tenant).Error)
	require.NoError(t, db.Create(&foreignTenant).Error)
	foreignShop := shop.Shop{TenantID: foreignTenant.ID, Platform: "mock", ShopName: "foreign", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, db.Create(&foreignShop).Error)
	productRow := product.Product{TenantID: tenant.ID, Source: "manual", Title: "owned", Status: product.StatusDraft}
	require.NoError(t, db.Create(&productRow).Error)
	require.NoError(t, db.Create(&productpublish.ProductPublication{
		TenantID: tenant.ID, ProductID: productRow.ID, ShopID: foreignShop.ID,
		Platform: "mock", Status: "published", PublishStatus: "published",
	}).Error)
	nextRetry := time.Now().UTC().Add(-time.Minute)
	task := ImageTask{
		TenantID: tenant.ID, ProductID: &productRow.ID, TaskType: TaskTypeGenerateScene, Provider: "test",
		Status: StatusRetrying, NextRetryAt: &nextRetry,
	}
	require.NoError(t, db.Create(&task).Error)

	hook := &retrySchedulerRedisHook{}
	queue := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	queue.AddHook(hook)
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	svc := &Service{DB: db, Redis: &rdb.Client{Client: queue}, QueueEnabled: true, AutoRetryEnabled: true}

	svc.flushImageDueRetries(context.Background(), nil)

	require.Zero(t, hook.pushes)
	var stored ImageTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusRetrying, stored.Status)
	require.NotNil(t, stored.NextRetryAt)
	require.Nil(t, stored.RetryEnqueuedAt)
}

func TestImageRetrySchedulerDoesNotEnqueueForMismatchedProductTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image_retry_product_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &product.Product{}, &ImageTask{}))
	tenant := admin.Tenant{Code: "image-product-mismatch", Name: "Image product mismatch", Status: admin.TenantActive, Version: 1}
	foreignTenant := admin.Tenant{Code: "image-product-foreign", Name: "Image product foreign", Status: admin.TenantActive, Version: 1}
	require.NoError(t, db.Create(&tenant).Error)
	require.NoError(t, db.Create(&foreignTenant).Error)
	foreignProduct := product.Product{TenantID: foreignTenant.ID, Source: "manual", Title: "foreign", Status: product.StatusDraft}
	require.NoError(t, db.Create(&foreignProduct).Error)
	nextRetry := time.Now().UTC().Add(-time.Minute)
	task := ImageTask{
		TenantID: tenant.ID, ProductID: &foreignProduct.ID, TaskType: TaskTypeGenerateScene, Provider: "test",
		Status: StatusRetrying, NextRetryAt: &nextRetry,
	}
	require.NoError(t, db.Create(&task).Error)

	hook := &retrySchedulerRedisHook{}
	queue := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	queue.AddHook(hook)
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	svc := &Service{
		DB: db, Redis: &rdb.Client{Client: queue}, QueueEnabled: true, AutoRetryEnabled: true,
	}

	svc.flushImageDueRetries(context.Background(), nil)

	require.Zero(t, hook.pushes)
	var stored ImageTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusRetrying, stored.Status)
	require.NotNil(t, stored.NextRetryAt)
	require.Nil(t, stored.RetryEnqueuedAt)
}

func (h *retrySchedulerRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *retrySchedulerRedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		switch cmd.Name() {
		case "ping":
			cmd.(*redis.StatusCmd).SetVal("PONG")
			return nil
		case "lpush":
			h.pushes++
			cmd.(*redis.IntCmd).SetVal(1)
			return nil
		default:
			return next(ctx, cmd)
		}
	}
}

func (h *retrySchedulerRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestImageRetrySchedulerDoesNotEnqueueForSuspendedTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:image_retry_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &ImageTask{}))
	tenant := admin.Tenant{Code: "paused-image-retry", Name: "Paused image retry", Status: admin.TenantSuspended, Version: 1}
	require.NoError(t, db.Create(&tenant).Error)
	nextRetry := time.Now().UTC().Add(-time.Minute)
	task := ImageTask{
		TenantID: tenant.ID, TaskType: TaskTypeGenerateScene, Provider: "test",
		Status: StatusRetrying, NextRetryAt: &nextRetry,
	}
	require.NoError(t, db.Create(&task).Error)

	hook := &retrySchedulerRedisHook{}
	queue := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	queue.AddHook(hook)
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	svc := &Service{
		DB: db, Redis: &rdb.Client{Client: queue}, QueueEnabled: true, AutoRetryEnabled: true,
	}

	svc.flushImageDueRetries(context.Background(), nil)

	require.Zero(t, hook.pushes)
	var stored ImageTask
	require.NoError(t, db.First(&stored, "id = ?", task.ID).Error)
	require.Equal(t, StatusRetrying, stored.Status)
	require.NotNil(t, stored.NextRetryAt)
	require.Nil(t, stored.RetryEnqueuedAt)
}

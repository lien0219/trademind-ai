package ordersync

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	"github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

type orderSyncTenantRedisHook struct{}

func (orderSyncTenantRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (orderSyncTenantRedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "lpush" {
			cmd.(*redis.IntCmd).SetVal(1)
			return nil
		}
		return next(ctx, cmd)
	}
}

func (orderSyncTenantRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func openOrderSyncTenantDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_sync_tenant_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &shop.Shop{}, &OrderSyncTask{}))
	return db
}

func TestBeginInlineOrderSyncTaskRequiresActiveOwnedShop(t *testing.T) {
	db := openOrderSyncTenantDB(t)
	active := admin.Tenant{Code: "order-inline-active", Name: "Order inline active", Status: admin.TenantActive, Version: 1}
	suspended := admin.Tenant{Code: "order-inline-suspended", Name: "Order inline suspended", Status: admin.TenantSuspended, Version: 1}
	require.NoError(t, db.Create(&active).Error)
	require.NoError(t, db.Create(&suspended).Error)
	activeShop := shop.Shop{TenantID: active.ID, Platform: "mock", ShopName: "active", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	foreignShop := shop.Shop{TenantID: suspended.ID, Platform: "mock", ShopName: "foreign", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, db.Create(&activeShop).Error)
	require.NoError(t, db.Create(&foreignShop).Error)
	activeTask := OrderSyncTask{TenantID: active.ID, ShopID: activeShop.ID, Platform: "mock", TaskType: TaskTypeOrderSync, Status: StatusPending, Mode: ModeIncremental}
	suspendedTask := OrderSyncTask{TenantID: suspended.ID, ShopID: foreignShop.ID, Platform: "mock", TaskType: TaskTypeOrderSync, Status: StatusPending, Mode: ModeIncremental}
	mismatchedTask := OrderSyncTask{TenantID: active.ID, ShopID: foreignShop.ID, Platform: "mock", TaskType: TaskTypeOrderSync, Status: StatusPending, Mode: ModeIncremental}
	require.NoError(t, db.Create(&activeTask).Error)
	require.NoError(t, db.Create(&suspendedTask).Error)
	require.NoError(t, db.Create(&mismatchedTask).Error)

	svc := &Service{DB: db}
	parent, cancel := context.WithCancel(context.Background())
	wctx, err := svc.beginInlineTask(parent, activeTask.ID)
	require.NoError(t, err)
	tc, err := security.RequireTenantContext(wctx)
	require.NoError(t, err)
	require.Equal(t, active.ID, tc.TenantID)
	require.Equal(t, activeShop.ID, tc.ShopScope[0])
	cancel()
	require.ErrorIs(t, wctx.Err(), context.Canceled)

	_, err = svc.beginInlineTask(context.Background(), suspendedTask.ID)
	require.ErrorIs(t, err, tasktenant.ErrTenantNotActive)
	_, err = svc.beginInlineTask(context.Background(), mismatchedTask.ID)
	require.ErrorIs(t, err, security.ErrTaskShopScopeMismatch)
}

func orderSyncTenantContext(tenantID int64, shopID uuid.UUID) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/shops/"+shopID.String()+"/sync-orders", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestCreateShopSyncFreezesTenantAndScopesShopLookup(t *testing.T) {
	platform.Bootstrap()
	db := openOrderSyncTenantDB(t)
	tenant := admin.Tenant{Code: "order-create-owner", Name: "Order create owner", Status: admin.TenantActive, Version: 1}
	require.NoError(t, db.Create(&tenant).Error)
	owned := shop.Shop{TenantID: tenant.ID, Platform: "mock", ShopName: "owned", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	foreign := shop.Shop{TenantID: tenant.ID + 1, Platform: "mock", ShopName: "foreign", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, db.Create(&owned).Error)
	require.NoError(t, db.Create(&foreign).Error)

	queue := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	queue.AddHook(orderSyncTenantRedisHook{})
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	svc := &Service{DB: db, Redis: &rdb.Client{Client: queue}, QueueEnabled: true}

	out, err := svc.CreateShopSync(orderSyncTenantContext(owned.TenantID, owned.ID), owned.ID, SyncOrdersBody{}, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	var stored OrderSyncTask
	require.NoError(t, db.First(&stored, "id = ?", out.ID).Error)
	require.Equal(t, owned.TenantID, stored.TenantID)
	require.Equal(t, owned.ID, stored.ShopID)

	_, err = svc.CreateShopSync(orderSyncTenantContext(owned.TenantID, foreign.ID), foreign.ID, SyncOrdersBody{}, nil)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	var count int64
	require.NoError(t, db.Model(&OrderSyncTask{}).Where("shop_id = ?", foreign.ID).Count(&count).Error)
	require.Zero(t, count)
}

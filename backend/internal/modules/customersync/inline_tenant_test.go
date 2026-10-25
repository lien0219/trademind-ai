package customersync

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	"gorm.io/gorm"
)

func openCustomerInlineTenantDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:customer_inline_tenant_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &shop.Shop{}, &CustomerMessageSyncTask{}))
	return db
}

func TestBeginInlineTaskUsesFrozenTenantAndFailsClosed(t *testing.T) {
	db := openCustomerInlineTenantDB(t)
	active := admin.Tenant{Code: "inline-active", Name: "Inline active", Status: admin.TenantActive, Version: 1}
	suspended := admin.Tenant{Code: "inline-suspended", Name: "Inline suspended", Status: admin.TenantSuspended, Version: 1}
	require.NoError(t, db.Create(&active).Error)
	require.NoError(t, db.Create(&suspended).Error)
	activeShop := shop.Shop{TenantID: active.ID, Platform: "mock", ShopName: "active", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	suspendedShop := shop.Shop{TenantID: suspended.ID, Platform: "mock", ShopName: "suspended", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, db.Create(&activeShop).Error)
	require.NoError(t, db.Create(&suspendedShop).Error)
	activeTask := CustomerMessageSyncTask{TenantID: active.ID, ShopID: activeShop.ID, Platform: "mock", TaskType: TaskTypeCustomerMessageSync, Status: StatusPending, Mode: ModeIncremental}
	suspendedTask := CustomerMessageSyncTask{TenantID: suspended.ID, ShopID: suspendedShop.ID, Platform: "mock", TaskType: TaskTypeCustomerMessageSync, Status: StatusPending, Mode: ModeIncremental}
	require.NoError(t, db.Create(&activeTask).Error)
	require.NoError(t, db.Create(&suspendedTask).Error)

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
}

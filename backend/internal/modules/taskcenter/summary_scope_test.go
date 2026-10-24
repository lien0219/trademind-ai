package taskcenter

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestSummaryCountsMarksOnlyForAuthorizedShops(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:taskcenter_summary_scope_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ordersync.OrderSyncTask{}, &TaskFailureMark{}, &shop.Shop{}))

	const tenantID int64 = 27
	allowedShopID := uuid.New()
	deniedShopID := uuid.New()
	allowedTaskID := uuid.New()
	deniedTaskID := uuid.New()
	now := time.Now().UTC()
	for _, task := range []ordersync.OrderSyncTask{
		{HardDeleteBase: model.HardDeleteBase{ID: allowedTaskID, CreatedAt: now, UpdatedAt: now}, TenantID: tenantID, ShopID: allowedShopID, Platform: "test", TaskType: "orders", Status: "failed", Mode: "manual"},
		{HardDeleteBase: model.HardDeleteBase{ID: deniedTaskID, CreatedAt: now, UpdatedAt: now}, TenantID: tenantID, ShopID: deniedShopID, Platform: "test", TaskType: "orders", Status: "failed", Mode: "manual"},
	} {
		require.NoError(t, db.Create(&task).Error)
	}
	for _, mark := range []TaskFailureMark{
		{TenantID: tenantID, TaskType: TaskTypeOrderSync, SourceID: allowedTaskID.String(), SourceTable: SourceTableOrderSyncTasks, MarkType: MarkIgnored},
		{TenantID: tenantID, TaskType: TaskTypeOrderSync, SourceID: deniedTaskID.String(), SourceTable: SourceTableOrderSyncTasks, MarkType: MarkIgnored},
	} {
		require.NoError(t, db.Create(&mark).Error)
	}

	summary, err := (&Service{DB: db}).Summary(context.Background(), ListFailureParams{
		TenantID: tenantID, TenantScopeSet: true, TaskType: TaskTypeOrderSync, IncludeMarked: true,
		RestrictShopScope: true, AllowedShopIDs: []uuid.UUID{allowedShopID},
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, summary.IgnoredCount)
	require.EqualValues(t, 0, summary.HandledCount)
}

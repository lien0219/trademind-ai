package customersync

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func openCustomerSyncScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:customer_sync_scope_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&shop.Shop{}, &CustomerMessageSyncTask{}))
	return db
}

func customerSyncScopeContext(tenantID int64, userID uuid.UUID, role string, grants ...adminperm.StoreGrant) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	c.Set(ctxkey.AdminID, userID.String())
	c.Set("adminperm.principal", &adminperm.Principal{
		UserID: userID, Role: role, Permissions: adminperm.PermissionsForRole(role), StoreGrants: grants,
	})
	return c
}

func customerSyncTask(shopID uuid.UUID, tenantID int64, status string) CustomerMessageSyncTask {
	return CustomerMessageSyncTask{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		TenantID:       tenantID,
		ShopID:         shopID,
		Platform:       "mock",
		TaskType:       TaskTypeCustomerMessageSync,
		Status:         status,
		Mode:           ModeIncremental,
	}
}

func TestCustomerSyncScopeRejectsUnownedAndCrossTenantTasks(t *testing.T) {
	db := openCustomerSyncScopeTestDB(t)
	allowed := shop.Shop{TenantID: 7, Platform: "mock", ShopName: "allowed", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	ungranted := shop.Shop{TenantID: 7, Platform: "mock", ShopName: "ungranted", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	foreign := shop.Shop{TenantID: 8, Platform: "mock", ShopName: "foreign", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	for _, row := range []*shop.Shop{&allowed, &ungranted, &foreign} {
		require.NoError(t, db.Create(row).Error)
	}
	for _, row := range []CustomerMessageSyncTask{
		customerSyncTask(allowed.ID, 7, StatusSuccess),
		customerSyncTask(ungranted.ID, 7, StatusSuccess),
		customerSyncTask(foreign.ID, 8, StatusSuccess),
	} {
		require.NoError(t, db.Create(&row).Error)
	}

	actorID := uuid.New()
	c := customerSyncScopeContext(7, actorID, adminperm.RoleOperator, adminperm.StoreGrant{
		StoreID: allowed.ID, PermissionScope: admin.StorePermScopeView,
	})
	svc := &Service{DB: db}

	res, err := svc.List(c, ListQuery{Page: 1, PageSize: 50})
	require.NoError(t, err)
	require.Equal(t, int64(1), res.Total)
	require.Len(t, res.Items, 1)
	require.Equal(t, allowed.ID, res.Items[0].ShopID)

	var allowedTask CustomerMessageSyncTask
	require.NoError(t, db.Where("shop_id = ?", allowed.ID).First(&allowedTask).Error)
	got, err := svc.GetDTO(c, allowedTask.ID)
	require.NoError(t, err)
	require.Equal(t, allowedTask.ID, got.ID)

	var ungrantedTask CustomerMessageSyncTask
	require.NoError(t, db.Where("shop_id = ?", ungranted.ID).First(&ungrantedTask).Error)
	var foreignTask CustomerMessageSyncTask
	require.NoError(t, db.Where("shop_id = ?", foreign.ID).First(&foreignTask).Error)
	for _, tc := range []struct {
		name string
		id   uuid.UUID
	}{
		{name: "same tenant without store grant", id: ungrantedTask.ID},
		{name: "different tenant", id: foreignTask.ID},
	} {
		t.Run("get "+tc.name, func(t *testing.T) {
			_, err := svc.GetDTO(c, tc.id)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		})
	}
}

func TestCustomerSyncScopeRejectsCreateAndRetryOutsideStoreScope(t *testing.T) {
	db := openCustomerSyncScopeTestDB(t)
	allowed := shop.Shop{TenantID: 7, Platform: "mock", ShopName: "allowed", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	ungranted := shop.Shop{TenantID: 7, Platform: "mock", ShopName: "ungranted", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	foreign := shop.Shop{TenantID: 8, Platform: "mock", ShopName: "foreign", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	for _, row := range []*shop.Shop{&allowed, &ungranted, &foreign} {
		require.NoError(t, db.Create(row).Error)
	}
	failedUnowned := customerSyncTask(ungranted.ID, 7, StatusFailed)
	failedForeign := customerSyncTask(foreign.ID, 8, StatusFailed)
	require.NoError(t, db.Create(&failedUnowned).Error)
	require.NoError(t, db.Create(&failedForeign).Error)

	actorID := uuid.New()
	c := customerSyncScopeContext(7, actorID, adminperm.RoleOperator, adminperm.StoreGrant{
		StoreID: allowed.ID, PermissionScope: admin.StorePermScopeOperate,
	})
	svc := &Service{DB: db}

	for _, tc := range []struct {
		name   string
		shopID uuid.UUID
		taskID uuid.UUID
	}{
		{name: "create same tenant without store grant", shopID: ungranted.ID, taskID: uuid.Nil},
		{name: "create different tenant", shopID: foreign.ID, taskID: uuid.Nil},
		{name: "retry same tenant without store grant", shopID: ungranted.ID, taskID: failedUnowned.ID},
		{name: "retry different tenant", shopID: foreign.ID, taskID: failedForeign.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.taskID == uuid.Nil {
				_, err := svc.CreateShopSync(c, tc.shopID, SyncCustomerMessagesBody{}, &actorID)
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
				return
			}
			_, err := svc.RetryFailed(c, tc.taskID, &actorID)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound)
			var row CustomerMessageSyncTask
			require.NoError(t, db.First(&row, "id = ?", tc.taskID).Error)
			require.Equal(t, StatusFailed, row.Status)
		})
	}
}

func TestCustomerSyncRoutesRequireCustomerPermissions(t *testing.T) {
	db := openCustomerSyncScopeTestDB(t)
	actorID := uuid.New()
	allowed := shop.Shop{TenantID: 7, Platform: "mock", ShopName: "allowed", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, db.Create(&allowed).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, int64(7))
		c.Set(ctxkey.AdminID, actorID.String())
		role := c.GetHeader("X-Test-Role")
		c.Set("adminperm.principal", &adminperm.Principal{
			UserID: actorID, Role: role, Permissions: adminperm.PermissionsForRole(role),
			StoreGrants: []adminperm.StoreGrant{{StoreID: allowed.ID, PermissionScope: admin.StorePermScopeView}},
		})
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	viewerReq := httptest.NewRequest(http.MethodGet, "/api/v1/customer/message-sync/tasks", nil)
	viewerReq.Header.Set("X-Test-Role", adminperm.RoleReviewer)
	viewerResp := httptest.NewRecorder()
	r.ServeHTTP(viewerResp, viewerReq)
	require.Equal(t, http.StatusForbidden, viewerResp.Code)

	readonlyReq := httptest.NewRequest(http.MethodPost, "/api/v1/customer/message-sync/tasks/"+uuid.NewString()+"/retry", nil)
	readonlyReq.Header.Set("X-Test-Role", adminperm.RoleReadonly)
	readonlyResp := httptest.NewRecorder()
	r.ServeHTTP(readonlyResp, readonlyReq)
	require.Equal(t, http.StatusForbidden, readonlyResp.Code)

	serviceCtx := customerSyncScopeContext(7, actorID, adminperm.RoleReviewer, adminperm.StoreGrant{
		StoreID: allowed.ID, PermissionScope: admin.StorePermScopeView,
	})
	_, err := (&Service{DB: db}).GetDTO(serviceCtx, uuid.New())
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

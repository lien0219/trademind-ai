package shop

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestShopReferenceHelpersEnforceTenantAndStoreScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:shop_scope_helpers_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Shop{}))

	allowed := Shop{TenantID: 7, Platform: "manual", ShopName: "allowed", Status: StatusActive, AuthStatus: AuthAuthorized}
	ungranted := Shop{TenantID: 7, Platform: "manual", ShopName: "ungranted", Status: StatusActive, AuthStatus: AuthAuthorized}
	foreign := Shop{TenantID: 8, Platform: "manual", ShopName: "foreign", Status: StatusActive, AuthStatus: AuthAuthorized}
	require.NoError(t, db.Create(&allowed).Error)
	require.NoError(t, db.Create(&ungranted).Error)
	require.NoError(t, db.Create(&foreign).Error)

	c := shopScopeContext(7, uuid.New(), adminperm.RoleOperator, []uuid.UUID{allowed.ID})
	svc := &Service{DB: db}

	ok, err := svc.Exists(c, allowed.ID)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = svc.Exists(c, ungranted.ID)
	require.NoError(t, err)
	require.False(t, ok, "an operator must not validate an ungranted shop reference")

	ok, err = svc.Exists(c, foreign.ID)
	require.NoError(t, err)
	require.False(t, ok, "a shop in another tenant must not validate")

	summary, err := svc.GetSummary(c, ungranted.ID)
	require.NoError(t, err)
	require.Nil(t, summary)

	rows, err := svc.BatchSummaries(c, []uuid.UUID{allowed.ID, ungranted.ID, foreign.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Contains(t, rows, allowed.ID)
	require.NotContains(t, rows, ungranted.ID)
	require.NotContains(t, rows, foreign.ID)
}

func shopScopeContext(tenantID int64, userID uuid.UUID, role string, storeIDs []uuid.UUID) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	c.Set(ctxkey.AdminID, userID.String())
	grants := make([]adminperm.StoreGrant, 0, len(storeIDs))
	for _, storeID := range storeIDs {
		grants = append(grants, adminperm.StoreGrant{StoreID: storeID, PermissionScope: admin.StorePermScopeView})
	}
	c.Set("adminperm.principal", &adminperm.Principal{
		UserID: userID, Role: role, Permissions: adminperm.PermissionsForRole(role), StoreGrants: grants,
	})
	return c
}

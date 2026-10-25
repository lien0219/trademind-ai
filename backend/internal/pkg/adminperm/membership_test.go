package adminperm

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func membershipPrincipalFixture(t *testing.T) (*gorm.DB, admin.AdminUser) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:membership_principal_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &tenantScopedShopFixture{}))
	u := admin.AdminUser{TenantID: 11, Username: admin.NewInternalUsername(), PasswordHash: "test-only", Role: RoleAdmin, Status: admin.StatusActive, TokenVersion: 1}
	require.NoError(t, db.Create(&u).Error)
	for _, id := range []int64{11, 22} {
		require.NoError(t, db.Create(&admin.Tenant{ID: id, Code: uuid.NewString(), Name: "test tenant", Status: admin.TenantActive}).Error)
	}
	return db, u
}

func membershipContext(userID uuid.UUID, tenantID int64) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.AdminID, userID.String())
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestLoadPrincipalUsesMembershipRoleInSelectedTenant(t *testing.T) {
	db, u := membershipPrincipalFixture(t)
	require.NoError(t, db.Create(&admin.TenantMembership{TenantID: 22, UserID: u.ID, Role: RoleReadonly, Status: admin.MembershipActive}).Error)
	p, err := LoadPrincipal(membershipContext(u.ID, 22), db)
	require.NoError(t, err)
	require.False(t, p.Disabled)
	require.Equal(t, RoleReadonly, p.Role)
	require.False(t, p.Can(PermUserManage))
}

func TestLoadPrincipalNeverFallsBackFromMissingOrDisabledMembership(t *testing.T) {
	for _, status := range []string{"missing", admin.MembershipDisabled, admin.MembershipRemoved} {
		t.Run(status, func(t *testing.T) {
			db, u := membershipPrincipalFixture(t)
			if status != "missing" {
				require.NoError(t, db.Create(&admin.TenantMembership{TenantID: 11, UserID: u.ID, Role: RoleAdmin, Status: status}).Error)
			}
			p, err := LoadPrincipal(membershipContext(u.ID, 11), db)
			require.NoError(t, err)
			require.True(t, p.Disabled)
			require.False(t, p.Can(PermUserManage))
		})
	}
}

func TestLoadPrincipalTenantZeroDoesNotGrantLegacyAdminInTenantSchema(t *testing.T) {
	db, u := membershipPrincipalFixture(t)
	p, err := LoadPrincipal(membershipContext(u.ID, 0), db)
	require.NoError(t, err)
	require.True(t, p.Disabled)
	require.False(t, p.IsAdmin())
}

func TestSystemPrincipalHasOnlyOperationalCapabilities(t *testing.T) {
	db, u := membershipPrincipalFixture(t)
	require.NoError(t, db.Model(&u).Update("tenant_id", 0).Error)
	p, err := LoadPrincipal(membershipContext(u.ID, 0), db)
	require.NoError(t, err)
	require.True(t, p.IsAdmin())
	require.True(t, p.SystemContext)
	require.True(t, p.Can(PermSettingsManage))
	require.True(t, p.Can(PermSecurityKeyRotate))
	require.True(t, p.Can(PermStoreView), "global runtime status reads use store.view as a control-plane capability")
	require.False(t, p.Can(PermProductView))
	require.False(t, p.Can(PermUserManage))
	require.False(t, p.CanViewStore(uuid.New()))
	require.False(t, p.CanOperateStore(uuid.New()))
}

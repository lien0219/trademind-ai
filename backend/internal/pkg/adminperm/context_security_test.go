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

func TestLoadPrincipalRejectsAuthenticatedUserFromAnotherTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:adminperm_tenant_mismatch_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))
	uid := uuid.New()
	user := admin.AdminUser{
		TenantID: 22, Username: admin.NewInternalUsername(),
		Email: "tenant22@example.test", PasswordHash: "test-only",
		Role: RoleAdmin, Status: admin.StatusActive, TokenVersion: 1,
	}
	user.ID = uid
	require.NoError(t, db.Create(&user).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.AdminID, uid.String())
	c.Set(ctxkey.TenantID, int64(11))

	p, err := LoadPrincipal(c, db)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.True(t, p.Disabled)
	require.False(t, p.IsAdmin())
	require.False(t, p.Can(PermUserManage))
}

func TestLoadPrincipalRejectsTenantZeroMismatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:adminperm_tenant_zero_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}))
	uid := uuid.New()
	user := admin.AdminUser{
		TenantID: 22, Username: admin.NewInternalUsername(),
		Email: "tenant22-root-check@example.test", PasswordHash: "test-only",
		Role: RoleAdmin, Status: admin.StatusActive, TokenVersion: 1,
	}
	user.ID = uid
	require.NoError(t, db.Create(&user).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.AdminID, uid.String())
	c.Set(ctxkey.TenantID, int64(0))

	p, err := LoadPrincipal(c, db)
	require.NoError(t, err)
	require.True(t, p.Disabled)
	require.False(t, p.Can(PermUserManage))
}

type tenantScopedShopFixture struct {
	ID       uuid.UUID `gorm:"type:char(36);primaryKey"`
	TenantID int64
}

func (tenantScopedShopFixture) TableName() string { return "shops" }

func TestLoadPrincipalIgnoresStoreGrantsFromAnotherTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:adminperm_grant_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &tenantScopedShopFixture{}))
	uid := uuid.New()
	user := admin.AdminUser{
		TenantID: 22, Username: admin.NewInternalUsername(),
		Email: "operator22@example.test", PasswordHash: "test-only",
		Role: RoleOperator, Status: admin.StatusActive, TokenVersion: 1,
	}
	user.ID = uid
	require.NoError(t, db.Create(&user).Error)
	foreign := tenantScopedShopFixture{ID: uuid.New(), TenantID: 99}
	require.NoError(t, db.Create(&foreign).Error)
	require.NoError(t, db.Create(&admin.UserStorePermission{UserID: uid, StoreID: foreign.ID, PermissionScope: admin.StorePermScopeOperate}).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.AdminID, uid.String())
	c.Set(ctxkey.TenantID, int64(22))

	p, err := LoadPrincipal(c, db)
	require.NoError(t, err)
	require.Empty(t, p.StoreGrants)
	require.False(t, p.CanOperateStore(foreign.ID))
}

func TestLoadPrincipalFailsClosedWhenStoreGrantLookupFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:adminperm_grant_db_error_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &tenantScopedShopFixture{}))
	user := admin.AdminUser{
		TenantID: 22, Username: admin.NewInternalUsername(),
		Email: "operator-error@example.test", PasswordHash: "test-only",
		Role: RoleOperator, Status: admin.StatusActive, TokenVersion: 1,
	}
	user.ID = uuid.New()
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Migrator().DropTable(&admin.UserStorePermission{}))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.AdminID, user.ID.String())
	c.Set(ctxkey.TenantID, int64(22))

	_, err = LoadPrincipal(c, db)
	require.Error(t, err)
}

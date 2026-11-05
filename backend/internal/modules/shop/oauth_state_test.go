package shop

import (
	"net/http"
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

func TestOAuthStateBindingRejectsChangedIdentityAndSecurityVersion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &ShopAuthToken{}))

	actor := admin.AdminUser{
		TenantID: 77, Username: admin.NewInternalUsername(), Email: "oauth-actor@example.test",
		PasswordHash: "fixture", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 3,
	}
	require.NoError(t, db.Create(&actor).Error)
	other := admin.AdminUser{
		TenantID: 77, Username: admin.NewInternalUsername(), Email: "oauth-other@example.test",
		PasswordHash: "fixture", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 3,
	}
	require.NoError(t, db.Create(&other).Error)

	service := &Service{DB: db}
	shopID := uuid.New()
	c := oauthStateContext(actor)
	binding, err := service.newOAuthStateBinding(c, shopID)
	require.NoError(t, err)
	require.Equal(t, oauthStateBinding{
		TenantID: 77, UserID: actor.ID, TokenVersion: 3, ShopID: shopID,
	}, binding)
	require.NoError(t, service.validateOAuthStateBinding(c, binding, shopID))

	require.Error(t, service.validateOAuthStateBinding(c, binding, uuid.New()), "a state cannot authorize another shop")
	require.Error(t, service.validateOAuthStateBinding(oauthStateContext(other), binding, shopID), "a state cannot be transferred to another user")

	require.NoError(t, db.Model(&admin.AdminUser{}).Where("id = ?", actor.ID).Update("token_version", 4).Error)
	require.Error(t, service.validateOAuthStateBinding(c, binding, shopID), "a state issued before permission revocation must fail")
}

func TestOAuthStateBindingRejectsDisabledActorAndTenantMismatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &ShopAuthToken{}))
	actor := admin.AdminUser{
		TenantID: 88, Username: admin.NewInternalUsername(), Email: "oauth-disabled@example.test",
		PasswordHash: "fixture", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1,
	}
	require.NoError(t, db.Create(&actor).Error)
	service := &Service{DB: db}
	shopID := uuid.New()
	c := oauthStateContext(actor)
	binding, err := service.newOAuthStateBinding(c, shopID)
	require.NoError(t, err)

	tenantMismatch := oauthStateContext(actor)
	tenantMismatch.Set(ctxkey.TenantID, int64(89))
	require.Error(t, service.validateOAuthStateBinding(tenantMismatch, binding, shopID))

	require.NoError(t, db.Model(&admin.AdminUser{}).Where("id = ?", actor.ID).Update("status", "disabled").Error)
	require.Error(t, service.validateOAuthStateBinding(c, binding, shopID))
	_, err = service.newOAuthStateBinding(c, shopID)
	require.Error(t, err)
}

func oauthStateContext(actor admin.AdminUser) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/shops/oauth/callback", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	c.Set(ctxkey.TenantID, actor.TenantID)
	return c
}

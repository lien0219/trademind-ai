package adminperm

import (
	"net/http"
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

func TestCustomRoleRevocationReevaluatesAcrossRequests(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin_role_runtime_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}))
	uid := uuid.New()
	u := admin.AdminUser{
		Username:      admin.NewInternalUsername(),
		TenantID:      17,
		Email:         "user@example.net",
		PasswordHash:  "test-only",
		Role:          "r_0123456789abcdef01234567",
		Status:        admin.StatusActive,
		TokenVersion:  1,
	}
	u.ID = uid
	require.NoError(t, db.Create(&u).Error)
	role := admin.RoleDefinition{
		TenantID:        17,
		Code:            u.Role,
		Name:            "客服",
		PermissionsJSON: "[\"customer.view\",\"customer.operate\"]",
		Enabled:         true,
	}
	require.NoError(t, db.Create(&role).Error)
	newCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		c.Set(ctxkey.AdminID, uid.String())
		c.Set(ctxkey.TenantID, int64(17))
		return c
	}
	first, err := LoadPrincipal(newCtx(), db)
	require.NoError(t, err)
	require.True(t, first.Can(PermCustomerOperate))
	require.False(t, first.Can(PermUserManage))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&role).Update("enabled", false).Error; err != nil {
			return err
		}
		return tx.Model(&admin.AdminUser{}).Where("id = ?", uid).
			UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
	}))
	second, err := LoadPrincipal(newCtx(), db)
	require.NoError(t, err)
	require.False(t, second.Can(PermCustomerOperate))
	require.True(t, second.Disabled)
}

package douyinpreflight

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

var (
	errPreflightUnauthenticated = errors.New("douyin preflight: authentication required")
	errPreflightRootTenant      = errors.New("douyin preflight: root tenant required")
	errPreflightForbidden       = errors.New("douyin preflight: permission denied")
	errPreflightAuthorization   = errors.New("douyin preflight: authorization unavailable")
)

// authorizePreflight requires a persisted authenticated principal because the
// preflight reads and writes global tenant-zero settings and operational data.
func authorizePreflight(c *gin.Context, db *gorm.DB, write bool) bool {
	p, err := preflightPrincipal(c, db)
	if err != nil {
		switch {
		case errors.Is(err, errPreflightUnauthenticated):
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "需要登录")
		case errors.Is(err, errPreflightRootTenant), errors.Is(err, errPreflightForbidden):
			adminperm.DenyPermission(c)
		default:
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "预检权限校验不可用")
		}
		return false
	}
	if write {
		if p.IsReadonly() || !p.Can(adminperm.PermSettingsManage) {
			if p.IsReadonly() {
				adminperm.DenyReadonly(c)
			} else {
				adminperm.DenySettings(c)
			}
			return false
		}
		return true
	}
	if !p.Can(adminperm.PermStoreView) {
		adminperm.DenyPermission(c)
		return false
	}
	return true
}

func preflightPrincipal(c *gin.Context, db *gorm.DB) (*adminperm.Principal, error) {
	if c == nil || c.Request == nil {
		return nil, errPreflightUnauthenticated
	}
	if db == nil {
		return nil, errPreflightAuthorization
	}
	adminID := preflightAdminID(c)
	if adminID == nil {
		return nil, errPreflightUnauthenticated
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, errPreflightUnauthenticated
	}
	if tenantID != 0 {
		return nil, errPreflightRootTenant
	}
	p, err := adminperm.LoadPrincipal(c, db)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errPreflightAuthorization, err)
	}
	if p == nil || p.Disabled || p.UserID != *adminID || strings.TrimSpace(p.Role) == "" {
		return nil, errPreflightForbidden
	}
	return p, nil
}

func preflightAdminID(c *gin.Context) *uuid.UUID {
	if c == nil {
		return nil
	}
	raw, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return nil
	}
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || id == uuid.Nil {
		return nil
	}
	return &id
}

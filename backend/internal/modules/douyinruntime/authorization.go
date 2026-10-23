package douyinruntime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

var (
	errRuntimeAuthentication = errors.New("douyinruntime: authentication required")
	errRuntimeRootTenant     = errors.New("douyinruntime: root tenant required")
	errRuntimePermission     = errors.New("douyinruntime: permission denied")
	errRuntimeMisconfigured  = errors.New("douyinruntime: authorization unavailable")
	errRuntimeReasonRequired = errors.New("reason is required")
)

// authorizeRuntime enforces the platform runtime boundary at the HTTP edge.
// Runtime settings and metrics are global, so only an authenticated root tenant
// principal may read them. Mutations additionally require settings.manage and
// a non-readonly principal.
func authorizeRuntime(c *gin.Context, db *gorm.DB, write bool) bool {
	p, err := runtimePrincipal(c, db)
	if err != nil {
		switch {
		case errors.Is(err, errRuntimeAuthentication):
			response.Fail(c, 401, response.CodeUnauthorized, "需要登录")
		case errors.Is(err, errRuntimeRootTenant), errors.Is(err, errRuntimePermission):
			if write && errors.Is(err, errRuntimePermission) {
				adminperm.DenySettings(c)
			} else {
				adminperm.DenyPermission(c)
			}
		default:
			response.Fail(c, 500, response.CodeInternalError, "运行态权限校验不可用")
		}
		return false
	}
	if write {
		if p.IsReadonly() {
			adminperm.DenyReadonly(c)
			return false
		}
		if !p.Can(adminperm.PermSettingsManage) {
			adminperm.DenySettings(c)
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

// runtimePrincipal is deliberately stricter than adminperm.LoadPrincipal's
// legacy internal-call fallback. HTTP requests must carry a valid authenticated
// admin and trusted tenant context before any permission is considered.
func runtimePrincipal(c *gin.Context, db *gorm.DB) (*adminperm.Principal, error) {
	if c == nil || c.Request == nil {
		return nil, errRuntimeAuthentication
	}
	if db == nil {
		return nil, errRuntimeMisconfigured
	}
	adminID := parseAdminID(c)
	if adminID == nil || *adminID == uuid.Nil {
		return nil, errRuntimeAuthentication
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, errRuntimeAuthentication
	}
	if tenantID != 0 {
		return nil, errRuntimeRootTenant
	}
	p, err := adminperm.LoadPrincipal(c, db)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errRuntimeMisconfigured, err)
	}
	if p == nil || p.Disabled || p.UserID != *adminID || strings.TrimSpace(p.Role) == "" {
		return nil, errRuntimePermission
	}
	return p, nil
}

func authorizeRuntimeMutation(c *gin.Context, db *gorm.DB) (*adminperm.Principal, *uuid.UUID, error) {
	p, err := runtimePrincipal(c, db)
	if err != nil {
		return nil, nil, err
	}
	if p.IsReadonly() || !p.Can(adminperm.PermSettingsManage) {
		return nil, nil, errRuntimePermission
	}
	adminID := parseAdminID(c)
	if adminID == nil {
		return nil, nil, errRuntimeAuthentication
	}
	return p, adminID, nil
}

func parseAdminID(c *gin.Context) *uuid.UUID {
	if c == nil {
		return nil
	}
	v, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	u, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil || u == uuid.Nil {
		return nil
	}
	return &u
}

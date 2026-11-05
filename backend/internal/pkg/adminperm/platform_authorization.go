package adminperm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

var (
	ErrPlatformAuthenticationRequired   = errors.New("platform authentication required")
	ErrPlatformAccessDenied             = errors.New("platform permission denied")
	ErrPlatformAuthorizationUnavailable = errors.New("platform authorization unavailable")
)

// ResolvePlatformPrincipal is the single authorization boundary for global
// platform operations from either the legacy tenant-zero control plane or
// the singleton Root account's validated positive-tenant session.
//
// Crucially, an enterprise administrator's role and PlatformOperator grant
// cannot substitute for the persisted singleton Root binding.
func ResolvePlatformPrincipal(c *gin.Context, db *gorm.DB) (*Principal, error) {
	if c == nil || c.Request == nil {
		return nil, ErrPlatformAuthenticationRequired
	}
	if db == nil {
		return nil, ErrPlatformAuthorizationUnavailable
	}
	rawID, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return nil, ErrPlatformAuthenticationRequired
	}
	idText, ok := rawID.(string)
	if !ok {
		return nil, ErrPlatformAuthenticationRequired
	}
	userID, err := uuid.Parse(strings.TrimSpace(idText))
	if err != nil || userID == uuid.Nil {
		return nil, ErrPlatformAuthenticationRequired
	}
	tenantID, err := TenantIDFromGin(c)
	if err != nil || tenantID < 0 {
		return nil, ErrPlatformAuthenticationRequired
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPlatformAuthorizationUnavailable, err)
	}
	if p == nil || p.Disabled || p.UserID != userID || p.TenantID != tenantID {
		return nil, ErrPlatformAccessDenied
	}
	if tenantID > 0 {
		root, err := IsRoot(c.Request.Context(), db, userID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPlatformAuthorizationUnavailable, err)
		}
		if !root {
			return nil, ErrPlatformAccessDenied
		}
		// LoadPrincipal has already checked that Root's positive tenant exists
		// and is active. No global business-data context is fabricated here.
		return p, nil
	}
	// Preserve the existing trusted tenant-zero system-administrator behavior.
	// A tenant's ordinary admin role can never satisfy this persisted identity.
	var user admin.AdminUser
	if err := db.WithContext(c.Request.Context()).Select("id", "tenant_id", "role", "status").
		First(&user, "id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPlatformAuthorizationUnavailable, err)
	}
	if !IsSystemAdministrator(&user) {
		return nil, ErrPlatformAccessDenied
	}
	return p, nil
}

// RequirePlatformPermission also enforces the requested operation permission.
// It does not bypass any downstream approval, data-scope, audit or runtime
// controls, and a plain PlatformOperator is not a singleton Root.
func RequirePlatformPermission(c *gin.Context, db *gorm.DB, permission string, write bool) bool {
	p, err := ResolvePlatformPrincipal(c, db)
	if err != nil {
		switch {
		case errors.Is(err, ErrPlatformAuthenticationRequired):
			response.Fail(c, 401, response.CodeUnauthorized, "需要登录")
		case errors.Is(err, ErrPlatformAccessDenied):
			DenySettings(c)
		default:
			response.Fail(c, 500, response.CodeInternalError, "平台权限校验不可用")
		}
		return false
	}
	if write && p.IsReadonly() {
		DenyReadonly(c)
		return false
	}
	if !p.Can(permission) {
		if permission == PermSettingsManage {
			DenySettings(c)
		} else {
			DenyPermission(c)
		}
		return false
	}
	return true
}

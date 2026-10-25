package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type switchTenantBody struct {
	TenantID *int64 `json:"tenantId" binding:"required,gte=0"`
}

// Tenants GET /api/v1/auth/tenants lists the caller's tenant memberships.
func (h *Handler) Tenants(c *gin.Context) {
	if h == nil || h.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "auth unavailable")
		return
	}
	uid, ok := currentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, ErrAuthenticationRequired)
		return
	}
	currentTenant, _ := adminperm.TenantIDFromGin(c)
	items := make([]gin.H, 0)
	var identity admin.AdminUser
	if err := h.DB.WithContext(c.Request.Context()).Select("id", "tenant_id", "role", "status").First(&identity, "id = ?", uid).Error; err != nil {
		response.HandleError(c, err)
		return
	}
	root, err := adminperm.IsRoot(c.Request.Context(), h.DB, uid)
	if err != nil {response.HandleError(c,err); return}
	if adminperm.IsSystemAdministrator(&identity) {
		items = append(items, gin.H{"id": 0, "code": "system", "name": "系统控制台", "status": admin.TenantActive, "role": adminperm.RoleAdmin})
	}
	registry, memberships := adminperm.TenantSchemaState(h.DB)
	if registry && memberships {
		var rows []struct {
			ID     int64
			Code   string
			Name   string
			Status string
			Role   string
		}
		if root {
			var all []admin.Tenant
			if err := h.DB.WithContext(c.Request.Context()).Where("status = ?",admin.TenantActive).Order("name ASC, id ASC").Find(&all).Error; err != nil {
				response.HandleError(c,err);return
			}
			for _,t := range all {items = append(items, gin.H{"id":t.ID,"code":t.Code,"name":t.Name,"status":t.Status,"role":adminperm.RoleAdmin})}
			response.OK(c,gin.H{"items":items,"currentTenantId":currentTenant})
			return
		}
		err := h.DB.WithContext(c.Request.Context()).Table("tenant_memberships AS m").
			Select("t.id, t.code, t.name, t.status, m.role").
			Joins("JOIN tenants AS t ON t.id = m.tenant_id").
			Where("m.user_id = ? AND m.status = ? AND t.status = ?", uid, admin.MembershipActive, admin.TenantActive).
			Order("t.name ASC, t.id ASC").Find(&rows).Error
		if err != nil {
			response.HandleError(c, err)
			return
		}
		for _, row := range rows {
			items = append(items, gin.H{"id": row.ID, "code": row.Code, "name": row.Name, "status": row.Status, "role": row.Role})
		}
	}
	response.OK(c, gin.H{"items": items, "currentTenantId": currentTenant})
}

// SwitchTenant POST /api/v1/auth/tenants/switch creates a new tenant-bound
// server session and invalidates the calling session or legacy bearer token.
func (h *Handler) SwitchTenant(c *gin.Context) {
	if h == nil || h.Sessions == nil || h.Sessions.DB == nil || h.Cfg == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "auth unavailable")
		return
	}
	uid, ok := currentUserID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, ErrAuthenticationRequired)
		return
	}
	var body switchTenantBody
	if err := c.ShouldBindJSON(&body); err != nil || body.TenantID == nil || *body.TenantID < 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant id")
		return
	}
	currentSessionID := uuid.Nil
	if raw, ok := c.Get(ctxkey.SessionID); ok {
		if value, ok := raw.(string); ok {
			currentSessionID, _ = uuid.Parse(strings.TrimSpace(value))
		}
	}
	currentVersion := 0
	if raw, ok := c.Get(ContextTokenVersionKey); ok {
		switch value := raw.(type) {
		case int:
			currentVersion = value
		case int64:
			currentVersion = int(value)
		}
	}
	result, err := h.Sessions.SwitchTenant(c.Request.Context(), uid, currentSessionID, currentVersion, *body.TenantID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == ErrSessionRevoked {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, ErrTenantAccessDenied)
			return
		}
		if strings.Contains(err.Error(), "tenant") || err.Error() == ErrTenantAccessDenied {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, ErrTenantAccessDenied)
			return
		}
		response.HandleError(c, err)
		return
	}
	out := gin.H{"token": result.AccessToken, "expiresAt": result.AccessExp.Unix(), "user": result.User}
	SetRefreshCookieResponse(c, h.Cfg, result.RefreshToken, h.Cfg.RefreshTokenTTL())
	if !h.Cfg.UsesSecureSession() {
		out["refreshToken"] = result.RefreshToken
	}
	response.OK(c, out)
}

// RegisterTenantRoutes mounts tenant access endpoints on an authenticated v1 group.
func (h *Handler) RegisterTenantRoutes(group *gin.RouterGroup) {
	if h == nil || group == nil {
		return
	}
	group.GET("/auth/tenants", h.Tenants)
	group.POST("/auth/tenants/switch", h.SwitchTenant)
}

package shop

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func requireShopOperateContext(c *gin.Context, db *gorm.DB, shopID uuid.UUID) (int64, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return 0, err
	}
	if !adminperm.RequireStoreOperate(c, db, shopID) {
		return 0, gorm.ErrRecordNotFound
	}
	return tenantID, nil
}

func requireShopTenantAdmin(c *gin.Context, db *gorm.DB) error {
	p, err := adminperm.LoadPrincipal(c, db)
	if err != nil {
		response.HandleError(c, err)
		return err
	}
	if p == nil || !p.IsAdmin() {
		adminperm.DenyPermission(c)
		return fmt.Errorf("global administrator required")
	}
	return nil
}

func requireShopPermission(h *Handler, permission string, operation string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.Svc == nil || h.Svc.DB == nil {
			c.Abort()
			response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, permission) {
			c.Abort()
			return
		}
		idText := strings.TrimSpace(c.Param("id"))
		if idText == "" {
			idText = strings.TrimSpace(c.Query("shopId"))
		}
		if idText != "" && operation != "" {
			id, err := uuid.Parse(idText)
			if err == nil && id != uuid.Nil {
				allowed := false
				switch operation {
				case "view":
					allowed = adminperm.RequireStoreView(c, h.Svc.DB, id)
				case "operate":
					allowed = adminperm.RequireStoreOperate(c, h.Svc.DB, id)
				}
				if !allowed {
					c.Abort()
					return
				}
			}
		}
		c.Next()
	}
}

func requireGlobalAdmin(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.Svc == nil || h.Svc.DB == nil {
			c.Abort()
			response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
			return
		}
		p, err := adminperm.LoadPrincipal(c, h.Svc.DB)
		if err != nil {
			c.Abort()
			response.HandleError(c, err)
			return
		}
		if p == nil || !p.IsAdmin() {
			c.Abort()
			adminperm.DenyPermission(c)
			return
		}
		c.Next()
	}
}

func requireDouyinOAuthStart(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(c.Query("shopId")) == "" {
			requireGlobalAdmin(h)(c)
			return
		}
		requireShopPermission(h, adminperm.PermStoreOperate, "operate")(c)
	}
}

func requirePlatformSettingsManage(h *Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.Svc == nil || h.Svc.DB == nil {
			c.Abort()
			response.Fail(c, 500, response.CodeInternalError, "shop service unavailable")
			return
		}
		if !adminperm.RequirePlatformPermission(c, h.Svc.DB, adminperm.PermSettingsManage, true) {
			c.Abort()
			return
		}
		c.Next()
	}
}

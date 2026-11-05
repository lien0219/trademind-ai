package worker

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves worker observability APIs.
type Handler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

// Monitor GET /workers/monitor
func (h *Handler) Monitor(c *gin.Context) {
	if h == nil || h.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "database unavailable")
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	principal, err := adminperm.LoadPrincipal(c, h.DB)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if principal.UserID == uuid.Nil || !principal.Can(adminperm.PermTaskRetry) {
		adminperm.DenyPermission(c)
		return
	}
	out, err := BuildMonitorResponse(c.Request.Context(), h.DB, h.Cfg, MonitorScope{
		TenantID: tenantID, UserID: principal.UserID, Admin: principal.IsAdmin(), StoreIDs: principal.AllowedStoreIDs(),
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "worker monitor query failed")
		return
	}
	response.OK(c, out)
}

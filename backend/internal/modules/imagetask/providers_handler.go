package imagetask

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
)

// ListProviders GET /api/v1/image/providers
func (h *Handler) ListProviders(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	if !requireImagePermission(c, h.Svc.DB, adminperm.PermProductView) {
		return
	}
	response.OK(c, imgprov.AllProviderCapabilities())
}

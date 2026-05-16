package worker

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
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
	out, err := BuildMonitorResponse(c.Request.Context(), h.DB, h.Cfg)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "worker monitor query failed")
		return
	}
	response.OK(c, out)
}

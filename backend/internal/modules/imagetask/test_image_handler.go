package imagetask

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
)

type testImageBody struct {
	Provider string            `json:"provider"`
	TestMode string            `json:"testMode"`
	Settings map[string]string `json:"settings"`
}

// TestImage POST /api/v1/settings/test-image
// Optional JSON settings lets the admin test unsaved form values; empty body uses saved settings.image only.
func (h *Handler) TestImage(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.Settings == nil {
		response.Fail(c, 500, response.CodeInternalError, "image settings unavailable")
		return
	}
	var body testImageBody
	_ = c.ShouldBindJSON(&body)
	m, err := h.Svc.Settings.PlainByGroup(c.Request.Context(), 0, "image")
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	m = MergeImagePlain(m, body.Settings)
	res := imgprov.TestConnection(c.Request.Context(), m, body.Provider, body.TestMode)
	if res != nil && !res.OK {
		if h.Svc.OpLog != nil {
			_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "test_image",
				Resource: "settings",
				Status:   "failed",
				Message:  res.Message,
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, res.Message)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "test_image",
			Resource: "settings",
			Status:   "success",
		})
	}
	response.OK(c, res)
}

package pricing

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes pricing HTTP API.
type Handler struct {
	Svc *Service
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

// Calculate POST /api/v1/pricing/calculate
func (h *Handler) Calculate(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "pricing unavailable")
		return
	}
	var body CalculateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.Calculate(c.Request.Context(), body)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// ApplyProduct POST /api/v1/products/:id/pricing/apply
func (h *Handler) ApplyProduct(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "pricing unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	var body ProductApplyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	if !body.Confirm {
		out, err := h.Svc.PreviewProduct(c.Request.Context(), pid, body)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.OK(c, out)
		return
	}
	out, err := h.Svc.ApplyProduct(c.Request.Context(), pid, body, adminUUID(c))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// BatchApply POST /api/v1/products/pricing/batch-apply
func (h *Handler) BatchApply(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "pricing unavailable")
		return
	}
	var body BatchApplyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	if !body.Confirm {
		out, err := h.Svc.BatchPreview(c.Request.Context(), body)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.OK(c, out)
		return
	}
	out, err := h.Svc.BatchApply(c.Request.Context(), body, adminUUID(c))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

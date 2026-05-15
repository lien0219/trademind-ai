package aiprompt

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler exposes prompt CRUD.
type Handler struct {
	Svc *Service
}

// List GET /api/v1/ai/prompts
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompts unavailable")
		return
	}
	rows, err := h.Svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// Create POST /api/v1/ai/prompts
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompts unavailable")
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Create(c.Request.Context(), body)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// Get GET /api/v1/ai/prompts/:id
func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompts unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	row, err := h.Svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, row)
}

// Put PUT /api/v1/ai/prompts/:id
func (h *Handler) Put(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompts unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body UpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Update(c.Request.Context(), id, body)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// Delete DELETE /api/v1/ai/prompts/:id
func (h *Handler) Delete(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompts unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.Delete(c.Request.Context(), id); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// Enable POST /api/v1/ai/prompts/:id/enable
func (h *Handler) Enable(c *gin.Context) {
	h.setEnabled(c, true)
}

// Disable POST /api/v1/ai/prompts/:id/disable
func (h *Handler) Disable(c *gin.Context) {
	h.setEnabled(c, false)
}

func (h *Handler) setEnabled(c *gin.Context, enabled bool) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompts unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	row, err := h.Svc.SetEnabled(c.Request.Context(), id, enabled)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, row)
}

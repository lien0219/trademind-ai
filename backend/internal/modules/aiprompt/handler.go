package aiprompt

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler exposes prompt CRUD.
type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

var errPromptAuditUnavailable = errors.New("ai prompt audit unavailable")

func runAuditedMutation[T any](h *Handler, c *gin.Context, action, resourceID, message string, mutate func(*gorm.DB) (T, error)) (value T, mutationErr, auditErr error) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil || h.OpLog == nil {
		return value, nil, errPromptAuditUnavailable
	}
	err := h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		value, err = mutate(tx)
		if err != nil {
			mutationErr = err
			return err
		}
		if err := h.OpLog.WriteTx(c, tx, operationlog.WriteOpts{
			Action: action, Resource: "ai_prompt", ResourceID: resourceID,
			Status: "success", Message: message,
		}); err != nil {
			auditErr = err
			return err
		}
		return nil
	})
	if err != nil && auditErr == nil && mutationErr == nil {
		mutationErr = err
	}
	return value, mutationErr, auditErr
}

func (h *Handler) requirePlatformAdmin(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompts unavailable")
		return false
	}
	return adminperm.RequirePlatformPermission(c, h.Svc.DB, adminperm.PermSettingsManage, true)
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
	row, err, auditErr := runAuditedMutation(h, c, "ai.prompt.create", "", "prompt created", func(tx *gorm.DB) (*AIPrompt, error) {
		return h.Svc.CreateTx(c.Request.Context(), tx, body)
	})
	if auditErr != nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompt audit unavailable")
		return
	}
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
	row, err, auditErr := runAuditedMutation(h, c, "ai.prompt.update", id.String(), "prompt updated", func(tx *gorm.DB) (*AIPrompt, error) {
		return h.Svc.UpdateTx(c.Request.Context(), tx, id, body)
	})
	if auditErr != nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompt audit unavailable")
		return
	}
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
	_, err, auditErr := runAuditedMutation(h, c, "ai.prompt.delete", id.String(), "prompt deleted", func(tx *gorm.DB) (struct{}, error) {
		return struct{}{}, h.Svc.DeleteTx(c.Request.Context(), tx, id)
	})
	if auditErr != nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompt audit unavailable")
		return
	}
	if err != nil {
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
	action := "ai.prompt.enable"
	if !enabled {
		action = "ai.prompt.disable"
	}
	row, err, auditErr := runAuditedMutation(h, c, action, id.String(), "prompt availability changed", func(tx *gorm.DB) (*AIPrompt, error) {
		return h.Svc.SetEnabledTx(c.Request.Context(), tx, id, enabled)
	})
	if auditErr != nil {
		response.Fail(c, 500, response.CodeInternalError, "ai prompt audit unavailable")
		return
	}
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

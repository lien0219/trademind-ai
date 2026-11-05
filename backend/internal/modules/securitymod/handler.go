package securitymod

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves /api/v1/security/* endpoints.
type Handler struct {
	Svc *Service
	DB  *gorm.DB
}

// RegisterRoutes mounts security routes on authed group.
func RegisterRoutes(authed *gin.RouterGroup, h *Handler) {
	if authed == nil || h == nil {
		return
	}
	g := authed.Group("/security")
	g.GET("/overview", h.SecurityOverview)
	var audit *operationlog.Service
	if h.Svc != nil {
		audit = h.Svc.OpLogs
	}
	keys := g.Group("/keys", audit.AuditFailures("key_rotation", "key_rotation", "id"))
	keys.POST("/rotation/prepare", h.RotationPrepare)
	keys.GET("/rotation/status", h.RotationStatus)
	keys.POST("/rotation/start", h.RotationStart)
	keys.GET("/rotation/:id", h.RotationGet)
	keys.GET("/rotation/:id/progress", h.RotationProgress)
	keys.POST("/rotation/:id/pause", h.RotationPause)
	keys.POST("/rotation/:id/resume", h.RotationResume)
	keys.POST("/rotation/:id/verify", h.RotationVerify)
	keys.GET("/references", h.KeyReferences)
	g.GET("/audit/integrity/status", h.AuditIntegrityStatus)
	g.POST("/audit/integrity/verify", h.AuditIntegrityVerify)
}

func (h *Handler) RotationPrepare(c *gin.Context) {
	if !h.requireGlobalKeyAdministrator(c) {
		return
	}
	var body struct {
		ConfirmPhrase string `json:"confirmPhrase"`
	}
	_ = c.ShouldBindJSON(&body)
	if strings.TrimSpace(body.ConfirmPhrase) != "ROTATE-KEYS-DRY-RUN" {
		response.Fail(c, 400, response.CodeBadRequest, "AUTH_REAUTHENTICATION_REQUIRED")
		return
	}
	var out *RotationStatus
	err := h.withRotationAudit(c, "prepare", func(svc *Service) (string, error) {
		var err error
		out, err = svc.PrepareRotation(c.Request.Context())
		return "", err
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) RotationStatus(c *gin.Context) {
	if !h.requireGlobalKeyAdministrator(c) {
		return
	}
	out, err := h.Svc.PrepareRotation(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) RotationStart(c *gin.Context) {
	if !h.requireGlobalKeyAdministrator(c) {
		return
	}
	var body struct {
		ConfirmPhrase string `json:"confirmPhrase"`
	}
	_ = c.ShouldBindJSON(&body)
	if strings.TrimSpace(body.ConfirmPhrase) != "ROTATE-KEYS-START" {
		response.Fail(c, 400, response.CodeBadRequest, "AUTH_REAUTHENTICATION_REQUIRED")
		return
	}
	startedBy, _ := uuid.Parse(c.GetString(ctxkey.AdminID))
	var job *KeyRotationJob
	err := h.withRotationAudit(c, "start", func(svc *Service) (string, error) {
		var err error
		job, err = svc.StartRotation(c.Request.Context(), startedBy, false)
		if err != nil {
			return "", err
		}
		return job.ID.String(), nil
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, job)
}

func (h *Handler) RotationGet(c *gin.Context) {
	if !h.requireGlobalKeyAdministrator(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	job, err := h.Svc.GetRotation(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, job)
}

func (h *Handler) RotationProgress(c *gin.Context) {
	h.RotationGet(c)
}

func (h *Handler) RotationPause(c *gin.Context) {
	if !h.requireGlobalKeyAdministrator(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.withRotationAudit(c, "pause", func(svc *Service) (string, error) {
		return id.String(), svc.PauseRotation(c.Request.Context(), id)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "rotation job not found or not running")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"paused": true})
}

func (h *Handler) RotationResume(c *gin.Context) {
	if !h.requireGlobalKeyAdministrator(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.withRotationAudit(c, "resume", func(svc *Service) (string, error) {
		return id.String(), svc.ResumeRotation(c.Request.Context(), id)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "rotation job not found or not paused")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"resumed": true})
}

func (h *Handler) RotationVerify(c *gin.Context) {
	if !h.requireGlobalKeyAdministrator(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var ok bool
	var counts []SecretReferenceCount
	err = h.withRotationAudit(c, "verify", func(svc *Service) (string, error) {
		var err error
		ok, counts, err = svc.VerifyRotation(c.Request.Context(), id)
		return id.String(), err
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": ok, "references": counts})
}

func (h *Handler) KeyReferences(c *gin.Context) {
	if !h.requireGlobalKeyAdministrator(c) {
		return
	}
	kr, err := h.Svc.keyRing()
	if err != nil {
		response.HandleError(c, err)
		return
	}
	counts, err := h.Svc.CountSecretReferencesByKeyID(c.Request.Context(), kr.PreviousKeyIDs())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": counts})
}

func (h *Handler) AuditIntegrityStatus(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermAuditRead) {
		return
	}
	n, err := h.Svc.VerifyAuditIntegrity(c.Request.Context(), 7)
	if err != nil {
		response.OK(c, gin.H{"ok": false, "checked": n})
		return
	}
	response.OK(c, gin.H{"ok": true, "checked": n})
}

func (h *Handler) AuditIntegrityVerify(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermAuditRead) {
		return
	}
	var body struct {
		Days int `json:"days"`
	}
	_ = c.ShouldBindJSON(&body)
	n, err := h.Svc.VerifyAuditIntegrity(c.Request.Context(), body.Days)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "audit chain verification failed")
		return
	}
	response.OK(c, gin.H{"ok": true, "checked": n})
}

func (h *Handler) SecurityOverview(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermConfigRead) {
		return
	}
	out, err := h.Svc.SecurityOverview(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if idStr, ok := c.Get(ctxkey.AdminID); ok {
		if uid, parseErr := uuid.Parse(strings.TrimSpace(idStr.(string))); parseErr == nil && uid != uuid.Nil {
			var sessionCount int64
			_ = h.DB.Model(&auth.AuthSession{}).Where("user_id = ? AND status = ?", uid, auth.SessionStatusActive).Count(&sessionCount).Error
			out["activeSessionCount"] = sessionCount
		}
	}
	response.OK(c, out)
}

package tenantmgmt

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler is a control-plane endpoint. It never binds tenant scope to an
// arbitrary request header and does not bypass application RBAC.
type Handler struct {
	Service *Service
}

func (h *Handler) actor(c *gin.Context) (uuid.UUID, bool) {
	if h == nil || h.Service == nil || h.Service.DB == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeInternalError, "tenant service unavailable")
		return uuid.Nil, false
	}
	raw, ok := c.Get(ctxkey.AdminID)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return uuid.Nil, false
	}
	idStr, castOK := raw.(string)
	if !castOK {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "platform operator required")
		return uuid.Nil, false
	}
	actor, err := uuid.Parse(strings.TrimSpace(idStr))
	if err != nil || actor == uuid.Nil || h.Service.requireOperator(c.Request.Context(), actor) != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "platform operator required")
		return uuid.Nil, false
	}
	return actor, true
}

func (h *Handler) List(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	var rows []admin.Tenant
	if err := h.Service.DB.WithContext(c.Request.Context()).Order("id DESC").Limit(200).Find(&rows).Error; err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

func (h *Handler) Create(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var in CreateTenantInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant request")
		return
	}
	t, err := h.Service.Create(c.Request.Context(), actor, in)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to create tenant")
		return
	}
	response.OK(c, t)
}

func (h *Handler) SetStatus(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant id")
		return
	}
	var in struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant status")
		return
	}
	if err := h.Service.SetStatus(c.Request.Context(), actor, id, in.Status); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "tenant not found")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to update tenant")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	r := g.Group("/platform/tenants")
	r.GET("", h.List)
	r.POST("", h.Create)
	r.PATCH("/:id/status", h.SetStatus)
}

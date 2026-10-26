package tenantconfig

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type Handler struct {
	DB    *gorm.DB
	Audit *operationlog.Service
}

func (h *Handler) identity(c *gin.Context, write bool) (int64, uuid.UUID, bool) {
	if h == nil || h.DB == nil {
		response.Fail(c, 503, response.CodeInternalError, "tenant settings unavailable")
		return 0, uuid.Nil, false
	}
	raw, ok := c.Get(ctxkey.AdminID)
	if !ok {
		response.Fail(c, 401, response.CodeUnauthorized, "authentication required")
		return 0, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(toString(raw)))
	if err != nil || id == uuid.Nil {
		response.Fail(c, 403, response.CodeForbidden, "invalid principal")
		return 0, uuid.Nil, false
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, 403, response.CodeForbidden, "active tenant required")
		return 0, uuid.Nil, false
	}
	p, err := adminperm.LoadPrincipal(c, h.DB)
	if err != nil {
		response.Fail(c, 503, response.CodeInternalError, "permission check unavailable")
		return 0, uuid.Nil, false
	}
	if p == nil || p.Disabled || p.SystemContext || p.UserID != id || p.TenantID != tid {
		response.Fail(c, 403, response.CodeForbidden, "tenant membership required")
		return 0, uuid.Nil, false
	}
	if write {
		if p.IsReadonly() || !p.Can(adminperm.PermSettingsManage) {
			response.Fail(c, 403, response.CodeForbidden, "tenant settings permission required")
			return 0, uuid.Nil, false
		}
		if err := adminperm.CheckTenantPermission(c.Request.Context(), h.DB, tid, id, adminperm.PermSettingsManage); err != nil {
			response.Fail(c, 403, response.CodeForbidden, "tenant settings permission required")
			return 0, uuid.Nil, false
		}
	}
	return tid, id, true
}
func toString(v any) string { s, _ := v.(string); return s }
func (h *Handler) Get(c *gin.Context) {
	tid, _, ok := h.identity(c, false)
	if !ok {
		return
	}
	s, err := Read(c.Request.Context(), h.DB, tid, c.Param("module"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "unable to read tenant settings")
		return
	}
	response.OK(c, s)
}

type updateBody struct {
	ExpectedVersion *int64          `json:"expectedVersion" binding:"required"`
	Values          json.RawMessage `json:"values" binding:"required"`
}

func (h *Handler) Update(c *gin.Context) {
	tid, actor, ok := h.identity(c, true)
	if !ok {
		return
	}
	var body updateBody
	if err := c.ShouldBindJSON(&body); err != nil || body.ExpectedVersion == nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid tenant settings request")
		return
	}
	s, err := Write(c.Request.Context(), h.DB, h.Audit, tid, c.Param("module"), body.Values, *body.ExpectedVersion, actor, "update")
	if errors.Is(err, ErrVersionConflict) {
		response.Fail(c, http.StatusConflict, response.CodeConflict, "settings version conflict; reload")
		return
	}
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "unable to save tenant settings")
		return
	}
	response.OK(c, s)
}
func (h *Handler) ListHistory(c *gin.Context) {
	tid, _, ok := h.identity(c, true)
	if !ok {
		return
	}
	rows, err := History(c.Request.Context(), h.DB, tid, c.Param("module"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "unable to list tenant settings history")
		return
	}
	response.OK(c, gin.H{"items": rows})
}

type restoreBody struct {
	TargetVersion   int64  `json:"targetVersion" binding:"required,min=1"`
	ExpectedVersion *int64 `json:"expectedVersion" binding:"required"`
}

func (h *Handler) Restore(c *gin.Context) {
	tid, actor, ok := h.identity(c, true)
	if !ok {
		return
	}
	var body restoreBody
	if err := c.ShouldBindJSON(&body); err != nil || body.ExpectedVersion == nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid restore request")
		return
	}
	s, err := Restore(c.Request.Context(), h.DB, h.Audit, tid, c.Param("module"), body.TargetVersion, *body.ExpectedVersion, actor)
	switch {
	case errors.Is(err, ErrVersionConflict):
		response.Fail(c, 409, response.CodeConflict, "settings version conflict; reload")
	case errors.Is(err, ErrRestoreNotFound):
		response.Fail(c, 404, response.CodeNotFound, "settings revision not found")
	case err != nil:
		response.Fail(c, 400, response.CodeBadRequest, "unable to restore tenant settings")
	default:
		response.OK(c, s)
	}
}

// Personal preferences are owned by the current authenticated membership,
// never by a caller-supplied tenant or user identifier.
func (h *Handler) GetPersonal(c *gin.Context) {
	tid,uid,ok:=h.identity(c,false);if !ok{return}
	s,err:=ReadPersonalPreferences(c.Request.Context(),h.DB,tid,uid)
	if err!=nil{response.Fail(c,503,response.CodeInternalError,"personal preferences unavailable");return}
	response.OK(c,s)
}

func (h *Handler) PutPersonal(c *gin.Context) {
	tid,uid,ok:=h.identity(c,false);if !ok{return}
	var in updateBody
	if err:=c.ShouldBindJSON(&in);err!=nil||in.ExpectedVersion==nil{
		response.Fail(c,400,response.CodeBadRequest,"invalid personal preferences request");return
	}
	s,err:=WritePersonalPreferences(c.Request.Context(),h.DB,tid,uid,in.Values,*in.ExpectedVersion)
	if errors.Is(err,ErrVersionConflict){response.Fail(c,409,response.CodeConflict,"personal preference version conflict");return}
	if err!=nil{response.Fail(c,400,response.CodeBadRequest,"personal preferences rejected");return}
	response.OK(c,s)
}

func Register(group *gin.RouterGroup, handler *Handler) {
	if group == nil || handler == nil {
		return
	}
	group.GET("/tenant/preferences",handler.GetPersonal)
	group.PUT("/tenant/preferences",handler.PutPersonal)
	group.GET("/tenant/settings/:module", handler.Get)
	group.PUT("/tenant/settings/:module", handler.Update)
	group.GET("/tenant/settings/:module/history", handler.ListHistory)
	group.POST("/tenant/settings/:module/restore", handler.Restore)
}

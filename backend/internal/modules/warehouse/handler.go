package warehouse

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

const maxJSONBody = int64(32 << 10)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func (h *Handler) authorize(c *gin.Context, permission string) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "warehouse unavailable")
		return 0, nil, false
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || !principal.Can(permission) || (permission == adminperm.PermWarehouseManage && principal.IsReadonly()) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "warehouse permission denied")
		return 0, nil, false
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context missing")
		return 0, nil, false
	}
	return tenantID, principal, true
}

func (h *Handler) List(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermWarehouseView)
	if !ok {
		return
	}
	rows, err := h.Svc.List(c.Request.Context(), tenantID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

func (h *Handler) Create(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseManage)
	if !ok {
		return
	}
	var in CreateInput
	if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Create(c.Request.Context(), tenantID, principalActor(principal), in)
	if err != nil {
		if errors.Is(err, ErrInvalidWarehouse) {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrWarehouseConflict) {
			response.Fail(c, 409, response.CodeBadRequest, err.Error())
			return
		}
		response.HandleError(c, err)
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{TenantID: tenantID, Action: "warehouse.create", Resource: "warehouse", ResourceID: row.ID.String(), Permission: adminperm.PermWarehouseManage, Status: "success"})
	}
	response.OK(c, row)
}

func (h *Handler) Update(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermWarehouseManage)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid warehouse id")
		return
	}
	var in UpdateInput
	if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Update(c.Request.Context(), tenantID, id, in)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidWarehouse):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, ErrWarehouseAbsent):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, err.Error())
		default:
			response.HandleError(c, err)
		}
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{TenantID: tenantID, Action: "warehouse.update", Resource: "warehouse", ResourceID: row.ID.String(), Permission: adminperm.PermWarehouseManage, Status: "success"})
	}
	response.OK(c, row)
}

func principalActor(principal *adminperm.Principal) *uuid.UUID {
	if principal == nil || principal.UserID == uuid.Nil {
		return nil
	}
	id := principal.UserID
	return &id
}

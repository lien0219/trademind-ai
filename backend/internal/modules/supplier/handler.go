package supplier

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"github.com/trademind-ai/trademind/backend/internal/pkg/mask"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const maxJSONBody = int64(32 << 10)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func (h *Handler) authorize(c *gin.Context, permission string) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "supplier unavailable")
		return 0, nil, false
	}
	p, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || p == nil || !p.Can(permission) || (permission == adminperm.PermSupplierManage && p.IsReadonly()) {
		response.Fail(c, 403, response.CodeForbidden, "supplier permission denied")
		return 0, nil, false
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, 403, response.CodeForbidden, "tenant context missing")
		return 0, nil, false
	}
	return tid, p, true
}

func (h *Handler) List(c *gin.Context) {
	tid, principal, ok := h.authorize(c, adminperm.PermSupplierView)
	if !ok {
		return
	}
	rows, err := h.Svc.List(c.Request.Context(), tid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if principal == nil || !principal.Can(adminperm.PermPIIReadFull) {
		for i := range rows {
			maskSupplierContact(&rows[i])
		}
	}
	response.OK(c, gin.H{"list": rows})
}

func (h *Handler) Create(c *gin.Context) {
	tid, p, ok := h.authorize(c, adminperm.PermSupplierManage)
	if !ok {
		return
	}
	var in CreateInput
	if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Create(c.Request.Context(), tid, supplierActor(p), in)
	if err != nil {
		if errors.Is(err, ErrInvalidSupplier) {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrSupplierConflict) {
			response.Fail(c, 409, response.CodeBadRequest, err.Error())
			return
		}
		response.HandleError(c, err)
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{TenantID: tid, Action: "supplier.create", Resource: "supplier", ResourceID: row.ID.String(), Permission: adminperm.PermSupplierManage, Status: "success"})
	}
	if p == nil || !p.Can(adminperm.PermPIIReadFull) {
		maskSupplierContact(row)
	}
	response.OK(c, row)
}

func maskSupplierContact(row *Supplier) {
	if row == nil {
		return
	}
	row.Phone = mask.Phone(row.Phone)
	row.Email = mask.Email(row.Email)
}

func supplierActor(principal *adminperm.Principal) *uuid.UUID {
	if principal == nil || principal.UserID == uuid.Nil {
		return nil
	}
	id := principal.UserID
	return &id
}

func (h *Handler) BindSKU(c *gin.Context) {
	tid, _, ok := h.authorize(c, adminperm.PermSupplierManage)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid supplier id")
		return
	}
	var in BindSKUInput
	if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.BindSKU(c.Request.Context(), tid, id, in)
	if err != nil {
		if errors.Is(err, ErrInvalidSupplier) {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "supplier or product SKU not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{TenantID: tid, Action: "supplier.sku.bind", Resource: "supplier_sku", ResourceID: row.ID.String(), Permission: adminperm.PermSupplierManage, Status: "success"})
	}
	response.OK(c, row)
}

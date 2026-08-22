package procurement

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

const maxProcurementJSONBody = int64(128 << 10)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func (h *Handler) authorize(c *gin.Context, permission string) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "procurement unavailable")
		return 0, nil, false
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || !principal.Can(permission) || (permission != adminperm.PermProcurementView && principal.IsReadonly()) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "procurement permission denied")
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
	tenantID, _, ok := h.authorize(c, adminperm.PermProcurementView)
	if !ok {
		return
	}
	result, err := h.Svc.List(c.Request.Context(), tenantID, queryPositiveInt(c, "page", 1), queryPositiveInt(c, "pageSize", 20))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Get(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermProcurementView)
	if !ok {
		return
	}
	id, ok := purchaseOrderID(c)
	if !ok {
		return
	}
	row, err := h.Svc.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) Create(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermProcurementManage)
	if !ok {
		return
	}
	var in CreatePurchaseOrderInput
	if err := httpapi.BindStrictJSON(c, &in, maxProcurementJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Create(c.Request.Context(), tenantID, procurementActor(principal), in)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	h.writeLog(c, tenantID, "procurement.purchase_order.create", row.ID, adminperm.PermProcurementManage, "")
	response.OK(c, row)
}

func (h *Handler) Submit(c *gin.Context) {
	h.transition(c, adminperm.PermProcurementManage, "submit")
}

func (h *Handler) Approve(c *gin.Context) {
	h.transition(c, adminperm.PermProcurementApprove, "approve")
}

func (h *Handler) Cancel(c *gin.Context) {
	h.transition(c, adminperm.PermProcurementManage, "cancel")
}

func (h *Handler) Close(c *gin.Context) {
	h.transition(c, adminperm.PermProcurementManage, "close")
}

func (h *Handler) transition(c *gin.Context, permission, action string) {
	tenantID, principal, ok := h.authorize(c, permission)
	if !ok {
		return
	}
	id, ok := purchaseOrderID(c)
	if !ok {
		return
	}
	var in TransitionInput
	if err := httpapi.BindStrictJSON(c, &in, maxProcurementJSONBody); err != nil || len([]rune(in.Reason)) > 500 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	var row *PurchaseOrder
	var err error
	switch action {
	case "submit":
		row, err = h.Svc.Submit(c.Request.Context(), tenantID, id, in.ExpectedRevision)
	case "approve":
		row, err = h.Svc.Approve(c.Request.Context(), tenantID, id, in.ExpectedRevision, procurementActor(principal))
	case "cancel":
		row, err = h.Svc.Cancel(c.Request.Context(), tenantID, id, in.ExpectedRevision)
	case "close":
		row, err = h.Svc.Close(c.Request.Context(), tenantID, id, in.ExpectedRevision)
	default:
		err = ErrInvalidTransition
	}
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	h.writeLog(c, tenantID, "procurement.purchase_order."+action, row.ID, permission, strings.TrimSpace(in.Reason))
	response.OK(c, row)
}

func (h *Handler) Receive(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermProcurementReceive)
	if !ok {
		return
	}
	id, ok := purchaseOrderID(c)
	if !ok {
		return
	}
	var in ReceivePurchaseOrderInput
	if err := httpapi.BindStrictJSON(c, &in, maxProcurementJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	result, err := h.Svc.Receive(c.Request.Context(), tenantID, id, procurementActor(principal), in)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	h.writeLog(c, tenantID, "procurement.purchase_order.receive", result.Receipt.ID, adminperm.PermProcurementReceive, "")
	response.OK(c, result)
}

func (h *Handler) writeLog(c *gin.Context, tenantID int64, action string, resourceID uuid.UUID, permission, message string) {
	if h.OpLog == nil {
		return
	}
	_ = h.OpLog.Write(c, operationlog.WriteOpts{
		TenantID: tenantID, Action: action, Resource: "purchase_order", ResourceID: resourceID.String(),
		Permission: permission, Status: "success", Message: message,
	})
}

func handleProcurementError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrPurchaseOrderAbsent):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrPurchaseReturnAbsent):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrReturnInvalidInput):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrRevisionConflict), errors.Is(err, ErrOverReceipt), errors.Is(err, ErrIdempotencyConflict),
		errors.Is(err, ErrReturnInvalidTransition), errors.Is(err, ErrReturnRevisionConflict), errors.Is(err, ErrReturnIdempotencyConflict),
		errors.Is(err, ErrOverReturn), errors.Is(err, ErrReturnInsufficientStock), errors.Is(err, ErrReturnDutyConflict):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func purchaseOrderID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid purchase order id")
		return uuid.Nil, false
	}
	return id, true
}

func procurementActor(principal *adminperm.Principal) *uuid.UUID {
	if principal == nil || principal.UserID == uuid.Nil {
		return nil
	}
	id := principal.UserID
	return &id
}

func queryPositiveInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Query(key)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

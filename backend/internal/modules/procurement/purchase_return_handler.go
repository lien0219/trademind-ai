package procurement

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func (h *Handler) ListPurchaseReturns(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermProcurementView)
	if !ok {
		return
	}
	var purchaseOrderID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("purchaseOrderId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid purchase order id")
			return
		}
		purchaseOrderID = &id
	}
	result, err := h.Svc.ListPurchaseReturns(c.Request.Context(), tenantID, queryPositiveInt(c, "page", 1), queryPositiveInt(c, "pageSize", 20), c.Query("status"), purchaseOrderID)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetPurchaseReturn(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermProcurementView)
	if !ok {
		return
	}
	id, ok := purchaseReturnID(c)
	if !ok {
		return
	}
	row, err := h.Svc.GetPurchaseReturn(c.Request.Context(), tenantID, id)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) ListReturnableReceiptItems(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermProcurementView)
	if !ok {
		return
	}
	id, ok := purchaseOrderID(c)
	if !ok {
		return
	}
	result, err := h.Svc.ListReturnableReceiptItems(c.Request.Context(), tenantID, id)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreatePurchaseReturn(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermProcurementManage)
	if !ok {
		return
	}
	var in CreatePurchaseReturnInput
	if err := httpapi.BindStrictJSON(c, &in, maxProcurementJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreatePurchaseReturn(c.Request.Context(), tenantID, procurementActor(principal), in)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	h.writePurchaseReturnLog(c, tenantID, "create", row.ID, adminperm.PermProcurementManage, row.Reason)
	response.OK(c, row)
}

func (h *Handler) SubmitPurchaseReturn(c *gin.Context) {
	h.purchaseReturnAction(c, adminperm.PermProcurementManage, "submit")
}

func (h *Handler) ApprovePurchaseReturn(c *gin.Context) {
	h.purchaseReturnAction(c, adminperm.PermProcurementApprove, "approve")
}

func (h *Handler) CompletePurchaseReturn(c *gin.Context) {
	h.purchaseReturnAction(c, adminperm.PermProcurementReturn, "complete")
}

func (h *Handler) CancelPurchaseReturn(c *gin.Context) {
	h.purchaseReturnAction(c, adminperm.PermProcurementManage, "cancel")
}

func (h *Handler) purchaseReturnAction(c *gin.Context, permission, action string) {
	tenantID, principal, ok := h.authorize(c, permission)
	if !ok {
		return
	}
	id, ok := purchaseReturnID(c)
	if !ok {
		return
	}
	var in PurchaseReturnActionInput
	if err := httpapi.BindStrictJSON(c, &in, maxProcurementJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	actor := procurementActor(principal)
	var row *PurchaseReturn
	var err error
	switch action {
	case "submit":
		row, err = h.Svc.SubmitPurchaseReturn(c.Request.Context(), tenantID, id, actor, in)
	case "approve":
		row, err = h.Svc.ApprovePurchaseReturn(c.Request.Context(), tenantID, id, actor, in)
	case "complete":
		row, err = h.Svc.CompletePurchaseReturn(c.Request.Context(), tenantID, id, actor, in)
	case "cancel":
		row, err = h.Svc.CancelPurchaseReturn(c.Request.Context(), tenantID, id, actor, in)
	default:
		err = ErrReturnInvalidTransition
	}
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	h.writePurchaseReturnLog(c, tenantID, action, row.ID, permission, strings.TrimSpace(in.Reason))
	response.OK(c, row)
}

func (h *Handler) writePurchaseReturnLog(c *gin.Context, tenantID int64, action string, resourceID uuid.UUID, permission, message string) {
	if h.OpLog == nil {
		return
	}
	_ = h.OpLog.Write(c, operationlog.WriteOpts{
		TenantID: tenantID, Action: "procurement.purchase_return." + action, Resource: "purchase_return", ResourceID: resourceID.String(),
		Permission: permission, Status: "success", Message: message,
	})
}

func purchaseReturnID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid purchase return id")
		return uuid.Nil, false
	}
	return id, true
}

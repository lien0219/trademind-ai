package salesreturn

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

func (h *Handler) ListRefundExecutions(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSalesReturnView)
	if !ok {
		return
	}
	query := RefundExecutionListQuery{
		TenantID: tenantID, Status: strings.TrimSpace(c.Query("status")),
		Page: positiveInt(c, "page", 1), PageSize: positiveInt(c, "pageSize", 20),
	}
	var valid bool
	if query.SalesReturnID, valid = optionalQueryUUID(c, "salesReturnId"); !valid {
		return
	}
	if query.OrderID, valid = optionalQueryUUID(c, "orderId"); !valid {
		return
	}
	if principal != nil && !principal.IsAdmin() {
		query.RestrictStoreScope = true
		query.AllowedShopIDs = principal.AllowedStoreIDs()
	}
	result, err := h.Svc.ListRefundExecutions(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetRefundExecution(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSalesReturnView)
	if !ok {
		return
	}
	id, ok := refundExecutionID(c)
	if !ok {
		return
	}
	row, err := h.Svc.GetRefundExecution(c.Request.Context(), tenantID, id)
	if err != nil {
		handleError(c, err)
		return
	}
	if !canAccessRefundExecution(principal, row, false) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, ErrRefundExecutionAbsent.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) CreateRefundExecution(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSalesReturnRefund)
	if !ok {
		return
	}
	salesReturnID, ok := returnID(c)
	if !ok {
		return
	}
	if principal != nil && !principal.IsAdmin() {
		shopID, err := h.Svc.SalesReturnShopID(c.Request.Context(), tenantID, salesReturnID)
		if err != nil {
			handleError(c, err)
			return
		}
		if shopID == nil || !principal.CanOperateStore(*shopID) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, ErrAbsent.Error())
			return
		}
	}
	var in CreateRefundExecutionInput
	if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreateRefundExecution(c.Request.Context(), tenantID, salesReturnID, actor(principal), in)
	if err != nil {
		handleError(c, err)
		return
	}
	h.writeRefundLog(c, tenantID, "create", row.ID, "")
	response.OK(c, row)
}

func (h *Handler) RecordRefundResult(c *gin.Context) {
	h.refundAction(c, refundActionRecordResult)
}

func (h *Handler) ConfirmRefundFromPlatform(c *gin.Context) {
	h.refundAction(c, refundActionConfirmPlatform)
}

func (h *Handler) CancelRefundExecution(c *gin.Context) {
	h.refundAction(c, refundActionCancel)
}

func (h *Handler) refundAction(c *gin.Context, action string) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSalesReturnRefund)
	if !ok {
		return
	}
	id, ok := refundExecutionID(c)
	if !ok {
		return
	}
	current, err := h.Svc.GetRefundExecution(c.Request.Context(), tenantID, id)
	if err != nil {
		handleError(c, err)
		return
	}
	if !canAccessRefundExecution(principal, current, true) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, ErrRefundExecutionAbsent.Error())
		return
	}
	var row *RefundExecution
	var message string
	switch action {
	case refundActionRecordResult:
		var in RecordRefundResultInput
		if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
			return
		}
		message = strings.TrimSpace(in.Reason)
		row, err = h.Svc.RecordRefundResult(c.Request.Context(), tenantID, id, actor(principal), in)
	case refundActionConfirmPlatform:
		var in ConfirmRefundFromPlatformInput
		if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
			return
		}
		message = strings.TrimSpace(in.Reason)
		row, err = h.Svc.ConfirmRefundFromPlatform(c.Request.Context(), tenantID, id, actor(principal), in)
	case refundActionCancel:
		var in ActionInput
		if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
			return
		}
		message = strings.TrimSpace(in.Reason)
		row, err = h.Svc.CancelRefundExecution(c.Request.Context(), tenantID, id, actor(principal), in)
	default:
		err = ErrRefundExecutionInvalidTransition
	}
	if err != nil {
		handleError(c, err)
		return
	}
	h.writeRefundLog(c, tenantID, action, row.ID, message)
	response.OK(c, row)
}

func (h *Handler) writeRefundLog(c *gin.Context, tenantID int64, action string, id uuid.UUID, message string) {
	if h.OpLog == nil {
		return
	}
	_ = h.OpLog.Write(c, operationlog.WriteOpts{
		TenantID: tenantID, Action: "sales_return.refund_execution." + action,
		Resource: "refund_execution", ResourceID: id.String(), Permission: adminperm.PermSalesReturnRefund,
		Status: "success", Message: message,
	})
}

func refundExecutionID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid refund execution id")
		return uuid.Nil, false
	}
	return id, true
}

func optionalQueryUUID(c *gin.Context, key string) (*uuid.UUID, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid "+key)
		return nil, false
	}
	return &id, true
}

func canAccessRefundExecution(principal *adminperm.Principal, row *RefundExecution, write bool) bool {
	if principal == nil || row == nil {
		return false
	}
	if principal.IsAdmin() {
		return true
	}
	if row.InternalShopID == nil || *row.InternalShopID == uuid.Nil {
		return false
	}
	if write {
		return principal.CanOperateStore(*row.InternalShopID)
	}
	return principal.CanViewStore(*row.InternalShopID)
}

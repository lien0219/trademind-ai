package salesreturn

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

const maxJSONBody = int64(128 << 10)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func (h *Handler) authorize(c *gin.Context, permission string) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "sales returns unavailable")
		return 0, nil, false
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || !principal.Can(permission) || (permission != adminperm.PermSalesReturnView && principal.IsReadonly()) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "sales return permission denied")
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
	tenantID, _, ok := h.authorize(c, adminperm.PermSalesReturnView)
	if !ok {
		return
	}
	var orderID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("orderId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid order id")
			return
		}
		orderID = &id
	}
	result, err := h.Svc.List(c.Request.Context(), tenantID, positiveInt(c, "page", 1), positiveInt(c, "pageSize", 20), c.Query("status"), c.Query("type"), orderID)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) ListPlatformReconciliation(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSalesReturnView)
	if !ok {
		return
	}
	query := PlatformAfterSaleListQuery{
		TenantID: tenantID, Page: positiveInt(c, "page", 1), PageSize: positiveInt(c, "pageSize", 20),
		Platform: strings.TrimSpace(c.Query("platform")), PlatformShopID: strings.TrimSpace(c.Query("platformShopId")),
		ExternalOrderID: strings.TrimSpace(c.Query("orderNo")), PlatformStatus: strings.ToLower(strings.TrimSpace(c.Query("platformStatus"))),
		ReconciliationStatus: strings.TrimSpace(c.Query("reconciliationStatus")),
	}
	if query.ReconciliationStatus != "" && !validPlatformReconciliationStatus(query.ReconciliationStatus) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid reconciliation status")
		return
	}
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid shop id")
			return
		}
		query.InternalShopID = &id
	}
	var err error
	if query.Start, err = parseTimeQuery(c.Query("start")); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid start time")
		return
	}
	if query.End, err = parseTimeQuery(c.Query("end")); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid end time")
		return
	}
	if query.Start != nil && query.End != nil && query.Start.After(*query.End) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid time range")
		return
	}
	if principal != nil && !principal.IsAdmin() {
		query.RestrictStoreScope = true
		query.AllowedShopIDs = principal.AllowedStoreIDs()
	}
	result, err := h.Svc.ListPlatformAfterSales(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetPlatformReconciliation(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSalesReturnView)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid platform after-sale id")
		return
	}
	row, err := h.Svc.GetPlatformAfterSale(c.Request.Context(), tenantID, id)
	if err != nil {
		handleError(c, err)
		return
	}
	if principal != nil && !principal.IsAdmin() && (row.InternalShopID == nil || !principal.CanViewStore(*row.InternalShopID)) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, ErrAbsent.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Get(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermSalesReturnView)
	if !ok {
		return
	}
	id, ok := returnID(c)
	if !ok {
		return
	}
	row, err := h.Svc.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) ListReturnableItems(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermSalesReturnView)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || orderID == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid order id")
		return
	}
	result, err := h.Svc.ListReturnableItems(c.Request.Context(), tenantID, orderID)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Create(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSalesReturnManage)
	if !ok {
		return
	}
	var in CreateInput
	if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Create(c.Request.Context(), tenantID, actor(principal), in)
	if err != nil {
		handleError(c, err)
		return
	}
	h.writeLog(c, tenantID, "create", row.ID, adminperm.PermSalesReturnManage, row.Reason)
	response.OK(c, row)
}

func (h *Handler) Submit(c *gin.Context) {
	h.action(c, adminperm.PermSalesReturnManage, "submit")
}

func (h *Handler) Approve(c *gin.Context) {
	h.action(c, adminperm.PermSalesReturnApprove, "approve")
}

func (h *Handler) Complete(c *gin.Context) {
	h.action(c, adminperm.PermSalesReturnReceive, "complete")
}

func (h *Handler) Cancel(c *gin.Context) {
	h.action(c, adminperm.PermSalesReturnManage, "cancel")
}

func (h *Handler) action(c *gin.Context, permission, actionName string) {
	tenantID, principal, ok := h.authorize(c, permission)
	if !ok {
		return
	}
	id, ok := returnID(c)
	if !ok {
		return
	}
	var in ActionInput
	if err := httpapi.BindStrictJSON(c, &in, maxJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	var row *SalesReturn
	var err error
	switch actionName {
	case "submit":
		row, err = h.Svc.Submit(c.Request.Context(), tenantID, id, actor(principal), in)
	case "approve":
		row, err = h.Svc.Approve(c.Request.Context(), tenantID, id, actor(principal), in)
	case "complete":
		row, err = h.Svc.Complete(c.Request.Context(), tenantID, id, actor(principal), in)
	case "cancel":
		row, err = h.Svc.Cancel(c.Request.Context(), tenantID, id, actor(principal), in)
	default:
		err = ErrInvalidTransition
	}
	if err != nil {
		handleError(c, err)
		return
	}
	h.writeLog(c, tenantID, actionName, row.ID, permission, strings.TrimSpace(in.Reason))
	response.OK(c, row)
}

func (h *Handler) writeLog(c *gin.Context, tenantID int64, actionName string, id uuid.UUID, permission, message string) {
	if h.OpLog == nil {
		return
	}
	_ = h.OpLog.Write(c, operationlog.WriteOpts{
		TenantID: tenantID, Action: "sales_return." + actionName, Resource: "sales_return", ResourceID: id.String(),
		Permission: permission, Status: "success", Message: message,
	})
}

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrAbsent), errors.Is(err, ErrRefundExecutionAbsent):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrRefundExecutionInvalidInput):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrRevisionConflict), errors.Is(err, ErrIdempotencyConflict), errors.Is(err, ErrOverReturn), errors.Is(err, ErrDutyConflict), errors.Is(err, ErrWarehouseUnavailable), errors.Is(err, ErrRefundExecutionExists), errors.Is(err, ErrRefundExecutionInvalidTransition), errors.Is(err, ErrRefundExecutionRevisionConflict), errors.Is(err, ErrRefundExecutionIdempotencyConflict), errors.Is(err, ErrRefundExecutionDutyConflict), errors.Is(err, ErrRefundPlatformFactConflict), errors.Is(err, ErrRefundPlatformFactPending):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func returnID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid sales return id")
		return uuid.Nil, false
	}
	return id, true
}

func actor(principal *adminperm.Principal) *uuid.UUID {
	if principal == nil || principal.UserID == uuid.Nil {
		return nil
	}
	id := principal.UserID
	return &id
}

func positiveInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Query(key)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func parseTimeQuery(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

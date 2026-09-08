package warehousefee

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	adminmodel "github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

const maxJSONBody = int64(32 << 10)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func (h *Handler) authorize(c *gin.Context, permission string, write bool) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "warehouse fee ledger unavailable")
		return 0, nil, false
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || !principal.Can(permission) || (write && principal.IsReadonly()) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "warehouse fee permission denied")
		return 0, nil, false
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context missing")
		return 0, nil, false
	}
	return tenantID, principal, true
}

func actorID(principal *adminperm.Principal) *uuid.UUID {
	if principal == nil || principal.UserID == uuid.Nil {
		return nil
	}
	id := principal.UserID
	return &id
}

func fulfillmentScope(principal *adminperm.Principal, write bool) order.WarehouseFeeScope {
	if principal == nil || principal.IsAdmin() {
		return order.WarehouseFeeScope{}
	}
	if !write {
		return order.WarehouseFeeScope{RestrictStoreScope: true, AllowedShopIDs: principal.AllowedStoreIDs()}
	}
	allowed := make([]uuid.UUID, 0, len(principal.StoreGrants))
	for _, grant := range principal.StoreGrants {
		scope := strings.ToLower(strings.TrimSpace(grant.PermissionScope))
		if grant.StoreID != uuid.Nil && (scope == adminmodel.StorePermScopeOperate || scope == adminmodel.StorePermScopeManage) {
			allowed = append(allowed, grant.StoreID)
		}
	}
	return order.WarehouseFeeScope{RestrictStoreScope: true, AllowedShopIDs: allowed}
}

func bindJSON(c *gin.Context, target any) bool {
	if err := httpapi.BindStrictJSON(c, target, maxJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return false
	}
	return true
}

func parseID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param(name)))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid warehouse fee id")
		return uuid.Nil, false
	}
	return id, true
}

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrConflict), errors.Is(err, ErrFulfillmentBlocked):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func queryUUID(c *gin.Context, name string) (*uuid.UUID, bool) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return nil, true
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid "+name)
		return nil, false
	}
	return &id, true
}

func queryPositiveInt(c *gin.Context, name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Query(name)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func (h *Handler) ListRateCards(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermWarehouseFeeView, false)
	if !ok {
		return
	}
	warehouseID, ok := queryUUID(c, "warehouseId")
	if !ok {
		return
	}
	includeInactive := strings.EqualFold(strings.TrimSpace(c.Query("includeInactive")), "true")
	rows, err := h.Svc.ListRateCards(c.Request.Context(), tenantID, warehouseID, includeInactive)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

func (h *Handler) GetRateCard(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermWarehouseFeeView, false)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	row, err := h.Svc.GetRateCardDetail(c.Request.Context(), tenantID, id)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) CreateRateCard(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeManage, true)
	if !ok {
		return
	}
	var input CreateRateCardInput
	if !bindJSON(c, &input) {
		return
	}
	row, err := h.Svc.CreateRateCard(c.Request.Context(), tenantID, actorID(principal), input)
	if err != nil {
		handleError(c, err)
		return
	}
	h.log(c, tenantID, adminperm.PermWarehouseFeeManage, "warehouse_fee.rate_card.create", "warehouse_fee_rate_card", row.ID)
	response.OK(c, row)
}

func (h *Handler) UpdateRateCard(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeManage, true)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var input UpdateRateCardInput
	if !bindJSON(c, &input) {
		return
	}
	row, err := h.Svc.UpdateRateCard(c.Request.Context(), tenantID, actorID(principal), id, input)
	if err != nil {
		handleError(c, err)
		return
	}
	h.log(c, tenantID, adminperm.PermWarehouseFeeManage, "warehouse_fee.rate_card.update", "warehouse_fee_rate_card", row.ID)
	response.OK(c, row)
}

func (h *Handler) ListCandidates(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeView, false)
	if !ok {
		return
	}
	warehouseID, ok := queryUUID(c, "warehouseId")
	if !ok {
		return
	}
	result, err := h.Svc.ListCandidates(c.Request.Context(), tenantID, fulfillmentScope(principal, false), order.WarehouseFeeFactQuery{
		Page: queryPositiveInt(c, "page", 1), PageSize: queryPositiveInt(c, "pageSize", 20),
		OrderNo: c.Query("orderNo"), WarehouseID: warehouseID,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Preview(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeManage, true)
	if !ok {
		return
	}
	var input PreviewInput
	if !bindJSON(c, &input) {
		return
	}
	result, err := h.Svc.Preview(c.Request.Context(), tenantID, fulfillmentScope(principal, true), input)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Confirm(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeManage, true)
	if !ok {
		return
	}
	var input ConfirmInput
	if !bindJSON(c, &input) {
		return
	}
	result, err := h.Svc.Confirm(c.Request.Context(), tenantID, fulfillmentScope(principal, true), actorID(principal), input)
	if err != nil {
		handleError(c, err)
		return
	}
	h.log(c, tenantID, adminperm.PermWarehouseFeeManage, "warehouse_fee.snapshot.confirm", "warehouse_fee_snapshot", result.Snapshot.ID)
	response.OK(c, result)
}

func (h *Handler) ListSnapshots(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeView, false)
	if !ok {
		return
	}
	warehouseID, ok := queryUUID(c, "warehouseId")
	if !ok {
		return
	}
	result, err := h.Svc.ListSnapshots(c.Request.Context(), tenantID, fulfillmentScope(principal, false), SnapshotListQuery{
		Page: queryPositiveInt(c, "page", 1), PageSize: queryPositiveInt(c, "pageSize", 20), OrderNo: c.Query("orderNo"), WarehouseID: warehouseID, Currency: c.Query("currency"),
	})
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetSnapshot(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeView, false)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.Svc.GetSnapshot(c.Request.Context(), tenantID, fulfillmentScope(principal, false), id)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateAdjustment(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeManage, true)
	if !ok {
		return
	}
	snapshotID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var input CreateAdjustmentInput
	if !bindJSON(c, &input) {
		return
	}
	result, err := h.Svc.CreateAdjustment(c.Request.Context(), tenantID, fulfillmentScope(principal, true), actorID(principal), snapshotID, input)
	if err != nil {
		handleError(c, err)
		return
	}
	h.log(c, tenantID, adminperm.PermWarehouseFeeManage, "warehouse_fee.adjustment.create", "warehouse_fee_adjustment", result.Adjustment.ID)
	response.OK(c, result)
}

func (h *Handler) ReverseAdjustment(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermWarehouseFeeManage, true)
	if !ok {
		return
	}
	snapshotID, ok := parseID(c, "id")
	if !ok {
		return
	}
	adjustmentID, ok := parseID(c, "adjustmentId")
	if !ok {
		return
	}
	var input ReverseAdjustmentInput
	if !bindJSON(c, &input) {
		return
	}
	result, err := h.Svc.ReverseAdjustment(c.Request.Context(), tenantID, fulfillmentScope(principal, true), actorID(principal), snapshotID, adjustmentID, input)
	if err != nil {
		handleError(c, err)
		return
	}
	h.log(c, tenantID, adminperm.PermWarehouseFeeManage, "warehouse_fee.adjustment.reverse", "warehouse_fee_adjustment", result.Adjustment.ID)
	response.OK(c, result)
}

func (h *Handler) log(c *gin.Context, tenantID int64, permission, action, resource string, resourceID uuid.UUID) {
	if h.OpLog == nil {
		return
	}
	_ = h.OpLog.Write(c, operationlog.WriteOpts{TenantID: tenantID, Action: action, Resource: resource, ResourceID: resourceID.String(), Permission: permission, Status: "success"})
}

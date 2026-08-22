package inventory

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

// Handler serves inventory ledger + outbound sync admin APIs.
type Handler struct {
	Svc *Service
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func atoiQ(c *gin.Context, key string, def int) int {
	s := strings.TrimSpace(c.Query(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func mapInventoryEnqueueErr(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, platformp.ErrManualInventorySyncUnsupported):
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return true
	case errors.Is(err, platformp.ErrInventorySyncNotImplemented):
		response.Fail(c, http.StatusNotImplemented, response.CodeBadRequest, err.Error())
		return true
	default:
		return false
	}
}

func parseBoolQuery(c *gin.Context, key string) bool {
	v := strings.TrimSpace(strings.ToLower(c.Query(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (h *Handler) requireInventoryWrite(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return false
	}
	if !adminperm.CanWriteInventory(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "只读账号不可执行库存写操作")
		return false
	}
	return true
}

func (h *Handler) requireInventoryRead(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "inventory unavailable")
		return false
	}
	if !adminperm.CanViewInventory(c, h.Svc.DB) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "inventory permission denied")
		return false
	}
	return true
}

// AdjustStock POST /products/:id/skus/:skuId/adjust-stock
func (h *Handler) AdjustStock(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("skuId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid sku id")
		return
	}
	var body AdjustStockBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	out, err := h.Svc.AdjustWarehouseStock(c.Request.Context(), tenantID, pid, sid, body, adminUUID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrInventoryIdempotency), errors.Is(err, ErrWarehouseLedgerMismatch):
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "SKU not found")
		default:
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		}
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: adminUUID(c), Action: "inventory.stock.adjust",
			Resource: "product_sku", ResourceID: sid.String(), Permission: adminperm.PermInventoryOperate,
			Status: "success", Message: fmt.Sprintf("warehouseId=%s aggregateStock=%d idempotentReplay=%t", out.WarehouseID, out.AggregateStock, out.IdempotentReplay),
		})
	}
	response.OK(c, out)
}

// ListWarehouseBalances GET /products/:id/skus/:skuId/warehouse-balances
func (h *Handler) ListWarehouseBalances(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "inventory unavailable")
		return
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || !principal.Can(adminperm.PermInventoryView) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "inventory permission denied")
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	productID, productErr := uuid.Parse(strings.TrimSpace(c.Param("id")))
	skuID, skuErr := uuid.Parse(strings.TrimSpace(c.Param("skuId")))
	if productErr != nil || skuErr != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid product or sku id")
		return
	}
	rows, err := h.Svc.ListWarehouseBalances(c.Request.Context(), tenantID, productID, skuID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "SKU not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// ReconcileWarehouseLedger GET /inventory/warehouse-ledger/reconciliation
func (h *Handler) ReconcileWarehouseLedger(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "inventory unavailable")
		return
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || !principal.Can(adminperm.PermInventoryView) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "inventory permission denied")
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	result, err := h.Svc.ReconcileWarehouseLedger(c.Request.Context(), tenantID, atoiQ(c, "page", 1), atoiQ(c, "pageSize", 20), c.Query("status"))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}

// MigrateLegacyStock POST /inventory/warehouse-ledger/migrate-legacy
func (h *Handler) MigrateLegacyStock(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body LegacyStockMigrationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	result, err := h.Svc.MigrateLegacyStock(c.Request.Context(), tenantID, adminUUID(c), body.Limit)
	if err != nil {
		if errors.Is(err, ErrWarehouseLedgerMismatch) {
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
			return
		}
		response.HandleError(c, err)
		return
	}
	if h.Svc.OpLog != nil && result.MigratedCount > 0 {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: adminUUID(c), Action: "inventory.legacy_stock.migrate",
			Resource: "warehouse", ResourceID: result.WarehouseID.String(), Permission: adminperm.PermInventoryOperate,
			Status: "success", Message: fmt.Sprintf("migratedCount=%d remainingCount=%d", result.MigratedCount, result.RemainingCount),
		})
	}
	response.OK(c, result)
}

func transferID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid warehouse transfer id")
		return uuid.Nil, false
	}
	return id, true
}

func handleTransferError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTransferInvalidInput):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrTransferAbsent):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrTransferTransition), errors.Is(err, ErrTransferRevision), errors.Is(err, ErrTransferIdempotency):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func (h *Handler) ListWarehouseTransfers(c *gin.Context) {
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	result, err := h.Svc.ListWarehouseTransfers(c.Request.Context(), tenantID, atoiQ(c, "page", 1), atoiQ(c, "pageSize", 20), c.Query("status"))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetWarehouseTransfer(c *gin.Context) {
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	id, ok := transferID(c)
	if !ok {
		return
	}
	row, err := h.Svc.GetWarehouseTransfer(c.Request.Context(), tenantID, id)
	if err != nil {
		handleTransferError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) CreateWarehouseTransfer(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body CreateWarehouseTransferBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreateWarehouseTransfer(c.Request.Context(), tenantID, adminUUID(c), body)
	if err != nil {
		handleTransferError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: adminUUID(c), Action: "inventory.warehouse_transfer.create", Resource: "warehouse_transfer", ResourceID: row.ID.String(), Permission: adminperm.PermInventoryOperate, Status: "success"})
	}
	response.OK(c, row)
}

func (h *Handler) transitionTransfer(c *gin.Context, action string) {
	permission := adminperm.PermInventoryOperate
	if action == "approve" {
		if h == nil || h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "inventory unavailable")
			return
		}
		principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
		if err != nil || principal == nil || !principal.Can(adminperm.PermInventoryApprove) || principal.IsReadonly() {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "inventory approval permission denied")
			return
		}
		permission = adminperm.PermInventoryApprove
	} else if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	id, ok := transferID(c)
	if !ok {
		return
	}
	var body WarehouseTransferActionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	var row *WarehouseTransfer
	switch action {
	case "submit":
		row, err = h.Svc.SubmitWarehouseTransfer(c.Request.Context(), tenantID, id, adminUUID(c), body)
	case "approve":
		row, err = h.Svc.ApproveWarehouseTransfer(c.Request.Context(), tenantID, id, adminUUID(c), body)
	case "dispatch":
		row, err = h.Svc.DispatchWarehouseTransfer(c.Request.Context(), tenantID, id, adminUUID(c), body)
	case "receive":
		row, err = h.Svc.ReceiveWarehouseTransfer(c.Request.Context(), tenantID, id, adminUUID(c), body)
	case "cancel":
		row, err = h.Svc.CancelWarehouseTransfer(c.Request.Context(), tenantID, id, adminUUID(c), body)
	default:
		err = ErrTransferInvalidInput
	}
	if err != nil {
		handleTransferError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: adminUUID(c), Action: "inventory.warehouse_transfer." + action, Resource: "warehouse_transfer", ResourceID: row.ID.String(), Permission: permission, Status: "success"})
	}
	response.OK(c, row)
}

func (h *Handler) SubmitWarehouseTransfer(c *gin.Context)   { h.transitionTransfer(c, "submit") }
func (h *Handler) ApproveWarehouseTransfer(c *gin.Context)  { h.transitionTransfer(c, "approve") }
func (h *Handler) DispatchWarehouseTransfer(c *gin.Context) { h.transitionTransfer(c, "dispatch") }
func (h *Handler) ReceiveWarehouseTransfer(c *gin.Context)  { h.transitionTransfer(c, "receive") }
func (h *Handler) CancelWarehouseTransfer(c *gin.Context)   { h.transitionTransfer(c, "cancel") }

func stocktakeParam(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param(key)))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid inventory stocktake id")
		return uuid.Nil, false
	}
	return id, true
}

func handleStocktakeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrStocktakeInvalidInput):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrStocktakeAbsent):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrStocktakeTransition), errors.Is(err, ErrStocktakeRevision), errors.Is(err, ErrStocktakeIdempotency), errors.Is(err, ErrStocktakeSnapshot):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "SKU not found")
	default:
		response.HandleError(c, err)
	}
}

func (h *Handler) ListInventoryStocktakes(c *gin.Context) {
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	result, err := h.Svc.ListInventoryStocktakes(c.Request.Context(), tenantID, atoiQ(c, "page", 1), atoiQ(c, "pageSize", 20), c.Query("status"))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetInventoryStocktake(c *gin.Context) {
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	id, ok := stocktakeParam(c, "id")
	if !ok {
		return
	}
	row, err := h.Svc.GetInventoryStocktake(c.Request.Context(), tenantID, id)
	if err != nil {
		handleStocktakeError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) CreateInventoryStocktake(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body CreateInventoryStocktakeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreateInventoryStocktake(c.Request.Context(), tenantID, adminUUID(c), body)
	if err != nil {
		handleStocktakeError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: adminUUID(c), Action: "inventory.stocktake.create", Resource: "inventory_stocktake", ResourceID: row.ID.String(), Permission: adminperm.PermInventoryOperate, Status: "success"})
	}
	response.OK(c, row)
}

func (h *Handler) UpdateInventoryStocktakeItem(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	stocktakeID, ok := stocktakeParam(c, "id")
	if !ok {
		return
	}
	itemID, ok := stocktakeParam(c, "itemId")
	if !ok {
		return
	}
	var body InventoryStocktakeItemBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.UpdateInventoryStocktakeItem(c.Request.Context(), tenantID, stocktakeID, itemID, adminUUID(c), body)
	if err != nil {
		handleStocktakeError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) transitionStocktake(c *gin.Context, action string) {
	permission := adminperm.PermInventoryOperate
	if action == "approve" {
		if h == nil || h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "inventory unavailable")
			return
		}
		principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
		if err != nil || principal == nil || !principal.Can(adminperm.PermInventoryApprove) || principal.IsReadonly() {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "inventory approval permission denied")
			return
		}
		permission = adminperm.PermInventoryApprove
	} else if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	id, ok := stocktakeParam(c, "id")
	if !ok {
		return
	}
	var body InventoryStocktakeActionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	var row *InventoryStocktake
	switch action {
	case "submit":
		row, err = h.Svc.SubmitInventoryStocktake(c.Request.Context(), tenantID, id, adminUUID(c), body)
	case "approve":
		row, err = h.Svc.ApproveInventoryStocktake(c.Request.Context(), tenantID, id, adminUUID(c), body)
	case "post":
		row, err = h.Svc.PostInventoryStocktake(c.Request.Context(), tenantID, id, adminUUID(c), body)
	case "cancel":
		row, err = h.Svc.CancelInventoryStocktake(c.Request.Context(), tenantID, id, adminUUID(c), body)
	default:
		err = ErrStocktakeInvalidInput
	}
	if err != nil {
		handleStocktakeError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: adminUUID(c), Action: "inventory.stocktake." + action, Resource: "inventory_stocktake", ResourceID: row.ID.String(), Permission: permission, Status: "success"})
	}
	response.OK(c, row)
}

func (h *Handler) SubmitInventoryStocktake(c *gin.Context)  { h.transitionStocktake(c, "submit") }
func (h *Handler) ApproveInventoryStocktake(c *gin.Context) { h.transitionStocktake(c, "approve") }
func (h *Handler) PostInventoryStocktake(c *gin.Context)    { h.transitionStocktake(c, "post") }
func (h *Handler) CancelInventoryStocktake(c *gin.Context)  { h.transitionStocktake(c, "cancel") }

// ListSKULogs GET /products/:id/skus/:skuId/inventory-logs
func (h *Handler) ListSKULogs(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("skuId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid sku id")
		return
	}
	page := atoiQ(c, "page", 1)
	ps := atoiQ(c, "pageSize", 20)
	res, err := h.Svc.ListSKUChangeLogs(c.Request.Context(), tenantID, pid, sid, page, ps)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListPublicationSKURows GET /products/:id/publication-skus
func (h *Handler) ListPublicationSKURows(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	var filter *uuid.UUID
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			filter = &u
		}
	}
	rows, err := h.Svc.ListPublicationSKUs(c.Request.Context(), tenantID, pid, filter)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// SyncPublicationSKU POST /product-publication-skus/:id/sync-inventory
func (h *Handler) SyncPublicationSKU(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body PublicationSKUSyncBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreatePublicationSKUInventoryTask(c, id, body, adminUUID(c))
	if err != nil {
		if mapInventoryEnqueueErr(c, err) {
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// BatchSyncProduct POST /products/:id/sync-inventory
func (h *Handler) BatchSyncProduct(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
		return
	}
	var body ProductBatchInventoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	list, err := h.Svc.CreateProductShopInventoryTasks(c, pid, body, adminUUID(c))
	if err != nil {
		if mapInventoryEnqueueErr(c, err) {
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"list": list})
}

// ListGlobalLogs GET /inventory/logs
func (h *Handler) ListGlobalLogs(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	q := GlobalLogsQuery{
		TenantID:   tenantID,
		Page:       atoiQ(c, "page", 1),
		PageSize:   atoiQ(c, "pageSize", 20),
		ChangeType: c.Query("changeType"),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSKUID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("orderId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.RefOrderID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	res, err := h.Svc.ListGlobalLogs(c.Request.Context(), q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListGlobalOrderEffects GET /inventory/effects
func (h *Handler) ListGlobalOrderEffects(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	q := OrderEffectsQuery{
		TenantID:   tenantID,
		Page:       atoiQ(c, "page", 1),
		PageSize:   atoiQ(c, "pageSize", 20),
		EffectType: c.Query("effectType"),
		Status:     c.Query("status"),
	}
	if raw := strings.TrimSpace(c.Query("orderId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.OrderID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSKUID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	res, err := h.Svc.ListOrderEffectsGlobal(c.Request.Context(), q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListCenter GET /inventory
func (h *Handler) ListCenter(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	q := CenterListQuery{
		TenantID:      tenantID,
		Cursor:        strings.TrimSpace(c.Query("cursor")),
		Limit:         atoiQ(c, "limit", 0),
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		Platform:      strings.TrimSpace(c.Query("platform")),
		StockStatus:   strings.TrimSpace(c.Query("stockStatus")),
		AlertStatus:   strings.TrimSpace(c.Query("alertStatus")),
		SKUBindStatus: strings.TrimSpace(c.Query("skuBindStatus")),
		SyncStatus:    strings.TrimSpace(c.Query("syncStatus")),
		HasException:  parseBoolQuery(c, "hasException"),
		Page:          atoiQ(c, "page", 1),
		PageSize:      atoiQ(c, "pageSize", 20),
	}
	q.UseCursor = q.Cursor != "" || q.Limit > 0
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSKUID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
		}
	}
	res, err := h.Svc.ListInventoryCenter(c.Request.Context(), q)
	if err != nil {
		if code := pagination.ErrorCode(err); code != "" {
			response.JSON(c, 400, response.CodeBadRequest, code, gin.H{"errorCode": code})
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"items":      res.Items,
		"nextCursor": res.NextCursor,
		"hasMore":    res.HasMore,
		"limit":      res.Limit,
		"list":       res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListAlerts GET /inventory/alerts
func (h *Handler) ListAlerts(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	q := AlertsListQuery{
		TenantID:      tenantID,
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		Platform:      strings.TrimSpace(c.Query("platform")),
		AlertType:     strings.TrimSpace(c.Query("alertType")),
		StockStatus:   strings.TrimSpace(c.Query("stockStatus")),
		OnlyPublished: parseBoolQuery(c, "onlyPublished"),
		IncludeNormal: parseBoolQuery(c, "includeNormal"),
		Page:          atoiQ(c, "page", 1),
		PageSize:      atoiQ(c, "pageSize", 20),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSKUID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
		}
	}
	res, err := h.Svc.ListInventoryAlerts(c.Request.Context(), q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListTasks GET /inventory-sync/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	q := ListQuery{
		Page:     atoiQ(c, "page", 1),
		PageSize: atoiQ(c, "pageSize", 20),
		Status:   c.Query("status"),
		Platform: c.Query("platform"),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("productSkuId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductSKUID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("batchId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.BatchID = &u
		}
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	res, err := h.Svc.ListTasks(c, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// GetTask GET /inventory-sync/tasks/:id
func (h *Handler) GetTask(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	out, err := h.Svc.GetDTO(c.Request.Context(), tid, id, uuid.Nil, "")
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// RetryTask POST /inventory-sync/tasks/:id/retry
func (h *Handler) RetryTask(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.RetryFailed(c, id, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

func parseUUIDQueryPtr(c *gin.Context, key string) *uuid.UUID {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil
	}
	u, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &u
}

// CreateInventorySyncBatch POST /inventory-sync/batches
func (h *Handler) CreateInventorySyncBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body CreateInventorySyncBatchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreateInventorySyncBatch(c.Request.Context(), tenantID, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ListInventorySyncBatches GET /inventory-sync/batches
func (h *Handler) ListInventorySyncBatches(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	q := InventorySyncBatchListQuery{
		TenantID:  tenantID,
		Source:    strings.TrimSpace(strings.ToLower(c.Query("source"))),
		Status:    strings.TrimSpace(strings.ToLower(c.Query("status"))),
		Platform:  strings.TrimSpace(strings.ToLower(c.Query("platform"))),
		ShopID:    parseUUIDQueryPtr(c, "shopId"),
		ProductID: parseUUIDQueryPtr(c, "productId"),
		Page:      atoiQ(c, "page", 1),
		PageSize:  atoiQ(c, "pageSize", 20),
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	res, err := h.Svc.ListInventorySyncBatches(c.Request.Context(), q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"items": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// GetInventorySyncBatch GET /inventory-sync/batches/:id
func (h *Handler) GetInventorySyncBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	recent := atoiQ(c, "recentTasks", 15)
	if recent > 50 {
		recent = 50
	}
	out, err := h.Svc.GetInventorySyncBatch(c.Request.Context(), tenantID, id, recent)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// ListInventorySyncBatchTasks GET /inventory-sync/batches/:id/tasks
func (h *Handler) ListInventorySyncBatchTasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid batch id")
		return
	}
	q := ListQuery{
		Page:         atoiQ(c, "page", 1),
		PageSize:     atoiQ(c, "pageSize", 20),
		Status:       c.Query("status"),
		Platform:     c.Query("platform"),
		ProductID:    parseUUIDQueryPtr(c, "productId"),
		ProductSKUID: parseUUIDQueryPtr(c, "productSkuId"),
		ShopID:       parseUUIDQueryPtr(c, "shopId"),
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.Start = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.End = &t
		}
	}
	res, err := h.Svc.ListInventorySyncBatchTasks(c, id, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// RetryInventorySyncBatchFailed POST /inventory-sync/batches/:id/retry-failed
func (h *Handler) RetryInventorySyncBatchFailed(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.RetryInventorySyncBatchFailed(c.Request.Context(), tenantID, id, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// RetryInventorySyncTasksBatch POST /inventory-sync/batches/retry-failed-tasks
func (h *Handler) RetryInventorySyncTasksBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body RetryInventorySyncTasksBatchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if len(body.TaskIDs) == 0 {
		response.Fail(c, 400, response.CodeBadRequest, "taskIds required")
		return
	}
	ids := make([]uuid.UUID, 0, len(body.TaskIDs))
	for _, raw := range body.TaskIDs {
		u, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid task id")
			return
		}
		ids = append(ids, u)
	}
	out, err := h.Svc.RetryInventorySyncTasksIntoBatch(c.Request.Context(), tenantID, ids, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// BatchPreviewStockSettings POST /inventory/stock-settings/batch-preview
func (h *Handler) BatchPreviewStockSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryRead(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body StockSettingsBatchPreviewBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.PreviewStockSettingsBatch(c.Request.Context(), tenantID, body)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// BatchUpdateStockSettings POST /inventory/stock-settings/batch-update
func (h *Handler) BatchUpdateStockSettings(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	if !h.requireInventoryWrite(c) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant context required")
		return
	}
	var body StockSettingsBatchUpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.BatchUpdateStockSettings(c.Request.Context(), tenantID, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

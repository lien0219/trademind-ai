package order

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/mask"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func (h *Handler) denyWrite(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return true
	}
	return !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermOrderOperate)
}

func (h *Handler) denyRead(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return true
	}
	return !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermOrderView)
}

// Handler exposes order HTTP routes.
type Handler struct {
	Svc *Service
	Inv *inventory.Service
}

func (h *Handler) enrichOrderInventoryMini(c *gin.Context, out *DetailDTO) {
	if h == nil || out == nil || h.Inv == nil {
		return
	}
	sum, err := h.Inv.SummarizeOrderInventoryEffects(c.Request.Context(), out.ID)
	if err != nil || sum == nil {
		return
	}
	out.InventorySummary = &InventoryUIMini{
		HasReservationSuccess: sum.HasReservationSuccess,
		HasReleaseSuccess:     sum.HasReleaseSuccess,
		HasDeductionSuccess:   sum.HasDeductionSuccess,
		HasRestoreSuccess:     sum.HasRestoreSuccess,
		FullyRestored:         sum.FullyRestored,
	}
}

func (h *Handler) rejectInventoryLockedItems(c *gin.Context, orderID uuid.UUID) bool {
	if h == nil || h.Inv == nil {
		return false
	}
	if h.Svc != nil {
		if _, err := h.Svc.Get(c, orderID); err != nil {
			if err == gorm.ErrRecordNotFound {
				response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			} else {
				response.HandleError(c, err)
			}
			return true
		}
	}
	hasInventory, err := h.Inv.HasSuccessfulOrderDeduction(c.Request.Context(), orderID)
	if err != nil {
		response.HandleError(c, err)
		return true
	}
	if hasInventory {
		response.Fail(c, 409, response.CodeBadRequest, "order items cannot change after inventory reservation or deduction")
		return true
	}
	return false
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

func clampOrderError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 480 {
		return msg[:480]
	}
	return msg
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

// List GET /orders
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyRead(c) {
		return
	}
	q := ListQuery{
		Page:                  atoiQ(c, "page", 1),
		PageSize:              atoiQ(c, "pageSize", 20),
		Cursor:                strings.TrimSpace(c.Query("cursor")),
		Limit:                 atoiQ(c, "limit", 0),
		Platform:              c.Query("platform"),
		OrderNo:               c.Query("orderNo"),
		CustomerName:          c.Query("customerName"),
		Keyword:               c.Query("keyword"),
		Status:                c.Query("status"),
		PaymentStatus:         c.Query("paymentStatus"),
		FulfillmentStatus:     c.Query("fulfillmentStatus"),
		SKUMatchStatus:        c.Query("skuMatchStatus"),
		InventoryDeductStatus: c.Query("inventoryDeductStatus"),
		SyncStatus:            c.Query("syncStatus"),
		HasException: strings.EqualFold(strings.TrimSpace(c.Query("hasException")), "true") ||
			strings.TrimSpace(c.Query("hasException")) == "1",
	}
	q.UseCursor = q.Cursor != "" || q.Limit > 0
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
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
	res, err := h.Svc.List(c, q)
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

// Create POST /orders
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	var pol inventory.StockOrderPolicy
	if h.Inv != nil {
		var policyErr error
		pol, policyErr = h.Inv.InventoryPolicy(c.Request.Context())
		if policyErr != nil {
			response.HandleError(c, policyErr)
			return
		}
	}
	shouldDed := h.Inv != nil && (body.DeductInventory || pol.AutoDeductManualOrders)
	platform := strings.ToLower(strings.TrimSpace(body.Platform))
	if platform == "" {
		platform = "manual"
	}
	if shouldDed && platform == "manual" && (body.WarehouseID == nil || *body.WarehouseID == uuid.Nil) {
		response.Fail(c, 400, response.CodeBadRequest, "warehouseId is required when applying inventory to a manual order")
		return
	}

	out, err := h.Svc.Create(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}

	if shouldDed {
		tenantID := out.TenantID
		sum, dex := h.Inv.DeductInventoryForOrder(c.Request.Context(), out.ID, inventory.OrderInventoryOptions{
			Reason:        "order_created",
			SyncPlatforms: body.SyncInventory,
			CreatedBy:     adminUUID(c),
			TenantID:      &tenantID,
		})
		if dex != nil {
			h.enrichOrderInventoryMini(c, out)
			if h.Svc.OpLog != nil {
				_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
					AdminUserID: adminUUID(c),
					Action:      "order.inventory_deduct",
					Resource:    "order",
					ResourceID:  out.ID.String(),
					Status:      "failed",
					Message:     "orderId=" + out.ID.String() + " error=" + clampOrderError(dex),
				})
			}
			response.JSON(c, http.StatusConflict, response.CodeBadRequest, "订单已创建，但库存处理失败，请在订单详情中重试库存操作", gin.H{
				"orderId":            out.ID,
				"order":              out,
				"inventoryDeduction": sum,
			})
			return
		}
	}
	h.enrichOrderInventoryMini(c, out)
	response.OK(c, out)
}

// Get GET /orders/:id
func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyRead(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.Get(c, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	h.enrichOrderInventoryMini(c, out)
	maskDetailPII(out)
	response.OK(c, out)
}

func maskDetailPII(out *DetailDTO) {
	if out == nil {
		return
	}
	if out.CustomerPhone != "" {
		out.CustomerPhone = mask.Phone(out.CustomerPhone)
	}
	if out.CustomerEmail != "" {
		out.CustomerEmail = mask.Email(out.CustomerEmail)
	}
}

// Update PUT /orders/:id
func (h *Handler) Update(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body UpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if body.ReplaceItems && h.rejectInventoryLockedItems(c, id) {
		return
	}
	var beforePtr *Order
	var pol inventory.StockOrderPolicy
	if h.Inv != nil {
		row, ierr := h.Svc.PeekOrderBeforeUpdate(c, id)
		if ierr != nil {
			if ierr == gorm.ErrRecordNotFound {
				response.Fail(c, 404, response.CodeNotFound, "not found")
			} else {
				response.HandleError(c, ierr)
			}
			return
		}
		beforePtr = row
		var policyErr error
		pol, policyErr = h.Inv.InventoryPolicy(c.Request.Context())
		if policyErr != nil {
			response.HandleError(c, policyErr)
			return
		}
	}
	if h.Inv != nil && beforePtr != nil && (body.SetWarehouseIDNil || (body.WarehouseID != nil && *body.WarehouseID != uuid.Nil)) {
		hasInventory, ierr := h.Inv.HasSuccessfulOrderDeduction(c.Request.Context(), id)
		if ierr != nil {
			response.HandleError(c, ierr)
			return
		}
		warehouseChanged := body.SetWarehouseIDNil || beforePtr.WarehouseID == nil || body.WarehouseID == nil || *beforePtr.WarehouseID != *body.WarehouseID
		if hasInventory && warehouseChanged {
			response.Fail(c, 409, response.CodeBadRequest, "warehouse cannot change after inventory reservation or deduction")
			return
		}
	}

	out, err := h.Svc.Update(c, id, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}

	if h.Inv != nil && beforePtr != nil {
		cur := Order{
			Status:            out.Status,
			PaymentStatus:     out.PaymentStatus,
			FulfillmentStatus: out.FulfillmentStatus,
		}
		if pol.AutoRestoreCancelledOrders && ShouldAutoRestoreStock(beforePtr, &cur) {
			syncPl := pol.AutoSyncPlatformInventoryAfterDeduct
			rsn := strings.TrimSpace(body.Status)
			if rsn == "" && strings.TrimSpace(body.PaymentStatus) != "" {
				rsn = "payment_" + strings.TrimSpace(body.PaymentStatus)
			}
			if rsn == "" {
				rsn = "order_status_auto"
			}
			if len(rsn) > 120 {
				rsn = rsn[:120]
			}
			if _, inventoryErr := h.Inv.RestoreInventoryForOrder(c.Request.Context(), id, inventory.OrderInventoryOptions{
				Reason:        rsn,
				SyncPlatforms: syncPl,
				CreatedBy:     adminUUID(c),
				TenantID:      &beforePtr.TenantID,
			}); inventoryErr != nil {
				response.Fail(c, http.StatusConflict, response.CodeBadRequest, "订单状态已更新，但库存补偿失败，请在订单详情中重试库存处理")
				return
			}
		} else if strings.TrimSpace(body.Status) != "" || strings.TrimSpace(body.PaymentStatus) != "" || strings.TrimSpace(body.FulfillmentStatus) != "" {
			platformAuto := !strings.EqualFold(strings.TrimSpace(beforePtr.Platform), "manual")
			if (platformAuto && pol.AutoDeductPlatformOrders) || (!platformAuto && pol.AutoDeductManualOrders) {
				if _, inventoryErr := h.Inv.DeductInventoryForOrder(c.Request.Context(), id, inventory.OrderInventoryOptions{
					Reason:        "order_status_auto",
					PlatformAuto:  platformAuto,
					SyncPlatforms: pol.AutoSyncPlatformInventoryAfterDeduct,
					CreatedBy:     adminUUID(c),
					TenantID:      &beforePtr.TenantID,
				}); inventoryErr != nil {
					response.Fail(c, http.StatusConflict, response.CodeBadRequest, "订单状态已更新，但库存处理失败，请在订单详情中重试库存操作")
					return
				}
			}
		}
	}

	h.enrichOrderInventoryMini(c, out)
	response.OK(c, out)
}

// Delete DELETE /orders/:id (soft delete)
func (h *Handler) Delete(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if h.Inv != nil {
		if _, ierr := h.Svc.Get(c, id); ierr != nil {
			if ierr == gorm.ErrRecordNotFound {
				response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			} else {
				response.HandleError(c, ierr)
			}
			return
		}
		uncompensated, ierr := h.Inv.HasUncompensatedOrderInventory(c.Request.Context(), id)
		if ierr != nil {
			response.HandleError(c, ierr)
			return
		}
		if uncompensated {
			response.Fail(c, 409, response.CodeBadRequest, "release or restore order inventory before deleting the order")
			return
		}
	}
	if err := h.Svc.Delete(c, id, adminUUID(c)); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// PostItem POST /orders/:id/items
func (h *Handler) PostItem(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if h.rejectInventoryLockedItems(c, oid) {
		return
	}
	var body OrderItemInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.AppendItem(c, oid, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// PutItem PUT /orders/:id/items/:itemId
func (h *Handler) PutItem(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	iid, err := uuid.Parse(strings.TrimSpace(c.Param("itemId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid itemId")
		return
	}
	if h.rejectInventoryLockedItems(c, oid) {
		return
	}
	var body OrderItemInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.PatchItem(c, oid, iid, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// DeleteItem DELETE /orders/:id/items/:itemId
func (h *Handler) DeleteItem(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	iid, err := uuid.Parse(strings.TrimSpace(c.Param("itemId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid itemId")
		return
	}
	if h.rejectInventoryLockedItems(c, oid) {
		return
	}
	if err := h.Svc.DeleteItem(c, oid, iid, adminUUID(c)); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// PostShipment POST /orders/:id/shipments
func (h *Handler) PostShipment(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body OrderShipmentInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.AppendShipment(c, oid, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// PutShipment PUT /orders/:id/shipments/:shipmentId
func (h *Handler) PutShipment(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("shipmentId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid shipmentId")
		return
	}
	var body OrderShipmentInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.PatchShipment(c, oid, sid, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// DeleteShipment DELETE /orders/:id/shipments/:shipmentId
func (h *Handler) DeleteShipment(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("shipmentId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid shipmentId")
		return
	}
	if err := h.Svc.DeleteShipment(c, oid, sid, adminUUID(c)); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

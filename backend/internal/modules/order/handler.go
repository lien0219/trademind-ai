package order

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/mask"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func (h *Handler) denyWrite(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		return false
	}
	if !adminperm.CanWriteOrders(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "当前账号为只读权限，无法执行此操作")
		return true
	}
	return false
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
		HasDeductionSuccess: sum.HasDeductionSuccess,
		HasRestoreSuccess:   sum.HasRestoreSuccess,
		FullyRestored:       sum.FullyRestored,
	}
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

// List GET /orders
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	q := ListQuery{
		Page:                  atoiQ(c, "page", 1),
		PageSize:              atoiQ(c, "pageSize", 20),
		Platform:              c.Query("platform"),
		OrderNo:               c.Query("orderNo"),
		CustomerName:          c.Query("customerName"),
		Keyword:               c.Query("keyword"),
		Status:                c.Query("status"),
		PaymentStatus:         c.Query("paymentStatus"),
		FulfillmentStatus:     c.Query("fulfillmentStatus"),
		SkuMatchStatus:        c.Query("skuMatchStatus"),
		InventoryDeductStatus: c.Query("inventoryDeductStatus"),
		SyncStatus:            c.Query("syncStatus"),
		HasException: strings.EqualFold(strings.TrimSpace(c.Query("hasException")), "true") ||
			strings.TrimSpace(c.Query("hasException")) == "1",
	}
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

// Create POST /orders
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.Create(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}

	var pol inventory.StockOrderPolicy
	if h.Inv != nil {
		pol, _ = h.Inv.InventoryPolicy(c.Request.Context())
	}
	shouldDed := h.Inv != nil && (body.DeductInventory || pol.AutoDeductManualOrders)
	if shouldDed {
		_, dex := h.Inv.DeductInventoryForOrder(c.Request.Context(), out.ID, inventory.OrderInventoryOptions{
			Reason:        "order_created",
			SyncPlatforms: body.SyncInventory,
			CreatedBy:     adminUUID(c),
		})
		if dex != nil {
			response.Fail(c, 400, response.CodeBadRequest, dex.Error())
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
	var beforePtr *Order
	if h.Inv != nil {
		row, ierr := h.Svc.PeekOrderBeforeUpdate(c, id)
		if ierr == nil && row != nil {
			beforePtr = row
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
		pol, _ := h.Inv.InventoryPolicy(c.Request.Context())
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
			_, _ = h.Inv.RestoreInventoryForOrder(c.Request.Context(), id, inventory.OrderInventoryOptions{
				Reason:        rsn,
				SyncPlatforms: syncPl,
				CreatedBy:     adminUUID(c),
			})
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
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
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
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
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

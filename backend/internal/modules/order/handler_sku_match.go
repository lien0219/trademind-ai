package order

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// GET /orders/:id/sku-matches
func (h *Handler) GetOrderSKUMatches(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if _, err := h.Svc.Get(c, oid); err != nil {
		response.HandleError(c, err)
		return
	}
	rows, err := h.Svc.ListSKUMatchRowsForOrder(c, oid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

type matchSkusBody struct {
	Overwrite bool `json:"overwrite"`
	Force     bool `json:"force"`
}

// POST /orders/:id/match-skus
func (h *Handler) PostMatchOrderSKUs(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body matchSkusBody
	_ = c.ShouldBindJSON(&body)
	sum, err := h.Svc.MatchOrderItemsForOrder(c.Request.Context(), oid, MatchOrderItemsOptions{
		Overwrite: body.Overwrite,
		Force:     body.Force,
		CreatedBy: adminUUID(c),
		Source:    "api",
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "order.sku_match.rebuild",
			Resource:    "order",
			ResourceID:  oid.String(),
			Status:      "success",
			Message: fmt.Sprintf("orderId=%s overwrite=%s force=%s",
				oid.String(), strconv.FormatBool(body.Overwrite), strconv.FormatBool(body.Force)),
		})
	}
	response.OK(c, gin.H{"summary": sum})
}

// GET /order-item-sku-matches
func (h *Handler) ListGlobalSKUMatches(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	q := SKUMatchListQuery{
		Page:        atoiQ(c, "page", 1),
		PageSize:    atoiQ(c, "pageSize", 20),
		Platform:    c.Query("platform"),
		MatchStatus: c.Query("matchStatus"),
		MatchType:   c.Query("matchType"),
	}
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
		}
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
	list, total, err := h.Svc.ListSKUMatchGlobal(c, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	response.OK(c, gin.H{
		"list": list,
		"pagination": gin.H{
			"page":       q.Page,
			"pageSize":   ps,
			"total":      total,
			"totalPages": pages,
		},
	})
}

type bindOrderItemSKUBody struct {
	ProductSKUID        string `json:"productSkuId"`
	DeductInventory     bool   `json:"deductInventory"`
	SyncInventory       bool   `json:"syncInventory"`
	CandidateConfidence *int   `json:"candidateConfidence"`
	CandidateSource     string `json:"candidateSource"`
}

// POST /order-items/:itemId/bind-sku
func (h *Handler) PostBindOrderItemSKU(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	itemID, err := uuid.Parse(strings.TrimSpace(c.Param("itemId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid itemId")
		return
	}
	var body bindOrderItemSKUBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	skuID, err := uuid.Parse(strings.TrimSpace(body.ProductSKUID))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid productSkuId")
		return
	}
	var line *OrderItem
	line, err = h.Svc.GetOrderItemByID(c, itemID)
	if err != nil {
		response.Fail(c, 404, response.CodeNotFound, "order item not found")
		return
	}
	if h.Inv != nil {
		pol, err := h.Inv.InventoryPolicy(c.Request.Context())
		if err != nil {
			response.HandleError(c, err)
			return
		}
		has, err := h.Inv.HasSuccessfulOrderDeduction(c.Request.Context(), line.OrderID)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		if has && !pol.AllowManualSkuBindAfterDeduct {
			response.Fail(c, 403, response.CodeForbidden, "manual sku bind after deduct is disabled in settings")
			return
		}
	}

	out, err := h.Svc.BindOrderItemSKU(c.Request.Context(), BindOrderItemSKUInput{
		OrderItemID:         itemID,
		ProductSKUID:        skuID,
		CandidateConfidence: body.CandidateConfidence,
		CandidateSource:     strings.TrimSpace(body.CandidateSource),
	}, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	var inv any
	if h.Inv != nil && body.DeductInventory {
		sum, derr := h.Inv.DeductInventoryForOrder(c.Request.Context(), line.OrderID, inventory.OrderInventoryOptions{
			Reason:        "manual_bind",
			SyncPlatforms: body.SyncInventory,
			CreatedBy:     adminUUID(c),
		})
		if derr != nil {
			response.Fail(c, 400, response.CodeBadRequest, derr.Error())
			return
		}
		inv = sum
	}
	response.OK(c, gin.H{"item": out, "inventoryDeduction": inv})
}

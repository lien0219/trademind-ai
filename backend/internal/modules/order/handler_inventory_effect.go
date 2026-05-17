package order

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type orderInvSyncBody struct {
	SyncInventory bool `json:"syncInventory"`
}

type orderRestoreBody struct {
	SyncInventory bool   `json:"syncInventory"`
	Reason        string `json:"reason"`
}

// POST /orders/:id/deduct-inventory
func (h *Handler) PostDeductInventory(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Inv == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body orderInvSyncBody
	_ = c.ShouldBindJSON(&body)

	sum, err := h.Inv.DeductInventoryForOrder(c.Request.Context(), id, inventory.OrderInventoryOptions{
		Reason:        "manual_api",
		SyncPlatforms: body.SyncInventory,
		CreatedBy:     adminUUID(c),
	})
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	out, ierr := h.Svc.Get(c, id)
	if ierr != nil {
		response.HandleError(c, ierr)
		return
	}
	h.enrichOrderInventoryMini(c, out)
	response.OK(c, gin.H{
		"order":              out,
		"inventoryDeduction": sum,
	})
}

// POST /orders/:id/restore-inventory
func (h *Handler) PostRestoreInventory(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Inv == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body orderRestoreBody
	_ = c.ShouldBindJSON(&body)
	rs := strings.TrimSpace(body.Reason)
	if rs == "" {
		rs = "manual_restore"
	}
	if len(rs) > 120 {
		rs = rs[:120]
	}

	sum, err := h.Inv.RestoreInventoryForOrder(c.Request.Context(), id, inventory.OrderInventoryOptions{
		Reason:        rs,
		SyncPlatforms: body.SyncInventory,
		CreatedBy:     adminUUID(c),
	})
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	out, ierr := h.Svc.Get(c, id)
	if ierr != nil {
		response.HandleError(c, ierr)
		return
	}
	h.enrichOrderInventoryMini(c, out)
	response.OK(c, gin.H{
		"order":                out,
		"inventoryRestoration": sum,
	})
}

// GET /orders/:id/inventory-effects
func (h *Handler) GetOrderInventoryEffects(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Inv == nil {
		response.Fail(c, 500, response.CodeInternalError, "orders unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if _, err := h.Svc.Get(c, id); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	page := atoiQ(c, "page", 1)
	ps := atoiQ(c, "pageSize", 50)
	list, ierr := h.Inv.ListOrderEffectsByOrder(c.Request.Context(), id, page, ps)
	if ierr != nil {
		response.HandleError(c, ierr)
		return
	}
	response.OK(c, gin.H{
		"list": list.Items,
		"pagination": gin.H{
			"page":       list.Page,
			"pageSize":   list.PageSize,
			"total":      list.Total,
			"totalPages": list.TotalPages,
		},
	})
}

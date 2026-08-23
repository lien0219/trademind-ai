package order

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// PostFulfill POST /orders/:id/fulfill
func (h *Handler) PostFulfill(c *gin.Context) {
	if h == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	if h.Svc == nil || h.Inv == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	var body FulfillOrderInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	result, err := h.Svc.FulfillOrder(c, h.Inv, id, body, adminUUID(c))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
		case errors.Is(err, ErrFulfillmentInProgress), errors.Is(err, ErrFulfillmentAlreadyCompleted), errors.Is(err, ErrFulfillmentNotPaid),
			errors.Is(err, inventory.ErrInsufficientSKUStock), errors.Is(err, inventory.ErrOrderInventoryState),
			errors.Is(err, inventory.ErrOrderWarehouseConflict), errors.Is(err, idempotency.ErrKeyConflict):
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
		case errors.Is(err, ErrFulfillmentIdempotencyRequired), errors.Is(err, ErrFulfillmentSKURequired),
			errors.Is(err, ErrFulfillmentShipmentRequired), errors.Is(err, ErrFulfillmentInputTooLong),
			errors.Is(err, ErrFulfillmentTrackingURLInvalid):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		default:
			response.HandleError(c, err)
		}
		return
	}
	if result.Order != nil {
		maskDetailPII(result.Order)
	}
	response.OK(c, result)
}

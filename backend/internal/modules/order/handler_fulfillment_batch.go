package order

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// PostFulfillBatch POST /orders/fulfillment-batch
func (h *Handler) PostFulfillBatch(c *gin.Context) {
	if h == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	if h.Svc == nil || h.Inv == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "batch fulfillment unavailable")
		return
	}
	var body BatchFulfillOrdersInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	result, err := h.Svc.FulfillOrders(c, h.Inv, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrBatchFulfillmentInputInvalid) {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}

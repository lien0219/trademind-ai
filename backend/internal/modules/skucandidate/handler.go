package skucandidate

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes read-only candidate APIs.
type Handler struct {
	Svc *Service
}

// GetByItem handles GET /order-items/:itemId/sku-candidates
func (h *Handler) GetByItem(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "sku candidates unavailable")
		return
	}
	idStr := strings.TrimSpace(c.Param("itemId"))
	iid, err := uuid.Parse(idStr)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid itemId")
		return
	}
	limit := atoiCandidateQuery(c.Query("limit"), 10)
	if limit > 20 {
		limit = 20
	}
	inc := strings.EqualFold(strings.TrimSpace(c.Query("includeLowConfidence")), "true")

	dto, err := h.Svc.SuggestForOrderItem(c.Request.Context(), iid, SuggestOpts{
		Limit:                limit,
		IncludeLowConfidence: inc,
	})
	if err != nil {
		response.Fail(c, 404, response.CodeNotFound, err.Error())
		return
	}
	response.OK(c, dto)
}

// PostBatch handles POST /orders/:id/sku-candidates/batch
func (h *Handler) PostBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "sku candidates unavailable")
		return
	}
	oidStr := strings.TrimSpace(c.Param("id"))
	oid, err := uuid.Parse(oidStr)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid order id")
		return
	}
	var body BatchRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	const maxIDs = 30
	if len(body.OrderItemIDs) > maxIDs {
		response.Fail(c, 400, response.CodeBadRequest, "too many orderItemIds")
		return
	}
	if body.Limit <= 0 {
		body.Limit = 10
	}
	if body.Limit > 20 {
		body.Limit = 20
	}

	dto, err := h.Svc.BatchForOrder(c.Request.Context(), oid, body)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, dto)
}

// Register attaches routes under Bearer /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/order-items/:itemId/sku-candidates", h.GetByItem)
	g.POST("/orders/:id/sku-candidates/batch", h.PostBatch)
}

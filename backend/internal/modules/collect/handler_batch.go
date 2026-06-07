package collect

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// CreateBatch POST /api/v1/collect/batches
func (h *Handler) CreateBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	var body CreateBatchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreateBatchAsync(c, body, collectAdminUUID(c))
	if err != nil {
		if errors.Is(err, ErrRedisQueueUnavailable) || errors.Is(err, ErrCollectQueueDisabled) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, err.Error())
			return
		}
		if errors.Is(err, ErrTaobaoTmallBatchLoginRequired) || errors.Is(err, ErrTaobaoTmallBatchVerifyRequired) {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ListBatches GET /api/v1/collect/batches
func (h *Handler) ListBatches(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	q := BatchListQuery{
		Page:     atoiCollectQP(c, "page", 1),
		PageSize: atoiCollectQP(c, "pageSize", 20),
		Status:   c.Query("status"),
		Source:   c.Query("source"),
		StartRFC: c.Query("start"),
		EndRFC:   c.Query("end"),
	}
	res, err := h.Svc.ListBatches(c, q)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
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

// GetBatch GET /api/v1/collect/batches/:id
func (h *Handler) GetBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.GetBatchDTO(c, id)
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

// ListBatchTasks GET /api/v1/collect/batches/:id/tasks
func (h *Handler) ListBatchTasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	batchID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	q := ListQuery{
		Page:     atoiCollectQP(c, "page", 1),
		PageSize: atoiCollectQP(c, "pageSize", 20),
		Status:   c.Query("status"),
	}
	res, err := h.Svc.ListBatchTasks(c, batchID, q)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
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

// RetryBatchFailed POST /api/v1/collect/batches/:id/retry-failed
func (h *Handler) RetryBatchFailed(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.RetryFailedBatchTasks(c, id, collectAdminUUID(c))
	if err != nil {
		if errors.Is(err, ErrRedisQueueUnavailable) || errors.Is(err, ErrCollectQueueDisabled) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, err.Error())
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

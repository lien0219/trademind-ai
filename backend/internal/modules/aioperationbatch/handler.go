package aioperationbatch

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves bulk AI batch HTTP API.
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

func atoiQP(c *gin.Context, key string, def int) int {
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

// CreateProductText POST /api/v1/ai/batches/product-text
func (h *Handler) CreateProductText(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
		return
	}
	var body CreateProductTextBatchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	batch, err := h.Svc.CreateProductTextBatch(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, batch)
}

// CreateProductImages POST /api/v1/ai/batches/product-images
func (h *Handler) CreateProductImages(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
		return
	}
	var body CreateProductImagesBatchBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	batch, err := h.Svc.CreateProductImagesBatch(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, batch)
}

// List GET /api/v1/ai/batches
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
		return
	}
	q := ListBatchesQuery{
		Page:          atoiQP(c, "page", 1),
		PageSize:      atoiQP(c, "pageSize", 20),
		OperationType: c.Query("operationType"),
		Status:        c.Query("status"),
	}
	if raw := strings.TrimSpace(c.Query("createdBy")); raw != "" {
		q.CreatedBy = &raw
	}
	if raw := strings.TrimSpace(c.Query("start")); raw != "" {
		q.Start = &raw
	}
	if raw := strings.TrimSpace(c.Query("end")); raw != "" {
		q.End = &raw
	}
	items, total, err := h.Svc.ListBatches(c, q)
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
	if pages == 0 && total > 0 {
		pages = 1
	}
	response.OK(c, gin.H{
		"list": items,
		"pagination": gin.H{
			"page":       q.Page,
			"pageSize":   ps,
			"total":      total,
			"totalPages": pages,
		},
	})
}

// Get GET /api/v1/ai/batches/:id
func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	batch, err := h.Svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}

	var recentAI []map[string]any
	var recentImg []imagetask.ImageTask
	switch batch.OperationType {
	case OperationTitleOptimize, OperationDescriptionGenerate:
		recentAI, _, _ = h.Svc.ListBatchAITasks(c, id, 1, 10)
	case OperationImageRemoveBackground, OperationImageGenerateScene, OperationImageReplaceBackground, OperationImageTranslateImageText:
		recentImg, _, _ = h.Svc.ListBatchImageTasks(c, id, 1, 10)
	}

	response.OK(c, gin.H{
		"batch":            batch,
		"recentAiTasks":    recentAI,
		"recentImageTasks": recentImg,
	})
}

// Tasks GET /api/v1/ai/batches/:id/tasks
func (h *Handler) Tasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	batch, err := h.Svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	page := atoiQP(c, "page", 1)
	ps := atoiQP(c, "pageSize", 20)

	switch batch.OperationType {
	case OperationTitleOptimize, OperationDescriptionGenerate:
		items, total, err := h.Svc.ListBatchAITasks(c, id, page, ps)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.OK(c, gin.H{"kind": "ai_tasks", "list": items, "pagination": pageBlock(page, ps, total)})
	case OperationImageRemoveBackground, OperationImageGenerateScene, OperationImageReplaceBackground, OperationImageTranslateImageText:
		items, total, err := h.Svc.ListBatchImageTasks(c, id, page, ps)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.OK(c, gin.H{"kind": "image_tasks", "list": items, "pagination": pageBlock(page, ps, total)})
	default:
		response.Fail(c, 400, response.CodeBadRequest, "unknown batch operationType")
	}
}

func pageBlock(page, ps int, total int64) gin.H {
	if ps < 1 {
		ps = 20
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return gin.H{
		"page":       page,
		"pageSize":   ps,
		"total":      total,
		"totalPages": pages,
	}
}

// RetryFailed POST /api/v1/ai/batches/:id/retry-failed
func (h *Handler) RetryFailed(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	batch, err := h.Svc.RetryFailed(c, id, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, batch)
}

// ApplyResults POST /api/v1/ai/batches/:id/apply-results
func (h *Handler) ApplyResults(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai batches unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body ApplyBatchResultsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	n, err := h.Svc.ApplyBatchResults(c, id, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"applied": n})
}

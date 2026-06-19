package aiproducttext

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves AI product text batch HTTP API.
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

// CheckBatch POST /api/v1/products/ai-text/batches/check
func (h *Handler) CheckBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	var body CheckBatchRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	out, err := h.Svc.CheckBatch(c.Request.Context(), body)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// CreateBatch POST /api/v1/products/ai-text/batches
func (h *Handler) CreateBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	var body CreateBatchRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	batch, err := h.Svc.CreateBatch(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, h.Svc.toBatchListItem(batch))
}

// ListBatches GET /api/v1/products/ai-text/batches
func (h *Handler) ListBatches(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	page := atoiQP(c, "page", 1)
	ps := atoiQP(c, "pageSize", 20)
	items, total, err := h.Svc.ListBatches(c.Request.Context(), page, ps)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if ps < 1 {
		ps = 20
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	response.OK(c, gin.H{
		"list": items,
		"pagination": gin.H{
			"page": page, "pageSize": ps, "total": total, "totalPages": pages,
		},
	})
}

// GetBatch GET /api/v1/products/ai-text/batches/:id
func (h *Handler) GetBatch(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的批次 ID")
		return
	}
	detail, err := h.Svc.GetBatchDetail(c.Request.Context(), id, c.Query("status"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "批次不存在")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, detail)
}

// RetryFailed POST /api/v1/products/ai-text/batches/:id/retry-failed
func (h *Handler) RetryFailed(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的批次 ID")
		return
	}
	batch, err := h.Svc.RetryFailed(c, id, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, h.Svc.toBatchListItem(batch))
}

// CancelPending POST /api/v1/products/ai-text/batches/:id/cancel-pending
func (h *Handler) CancelPending(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的批次 ID")
		return
	}
	n, err := h.Svc.CancelPending(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"cancelled": n})
}

// RegenerateItem POST /api/v1/products/ai-text/items/:id/regenerate
func (h *Handler) RegenerateItem(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的子项 ID")
		return
	}
	item, err := h.Svc.RegenerateItem(c, id, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, item)
}

// UpdateEditedText POST /api/v1/products/ai-text/items/:id/update-edited-text
func (h *Handler) UpdateEditedText(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的子项 ID")
		return
	}
	var body UpdateEditedTextBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	if err := h.Svc.UpdateEditedText(c.Request.Context(), id, body.EditedText); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// ApplyItem POST /api/v1/products/ai-text/items/:id/apply
func (h *Handler) ApplyItem(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的子项 ID")
		return
	}
	var body ApplyItemBody
	_ = c.ShouldBindJSON(&body)
	result, err := h.Svc.ApplyItem(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "子项不存在")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if result.Status == ItemConflict {
		response.JSON(c, 409, response.CodeBadRequest, result.ErrorMessage, gin.H{"errorCode": "AI_CONTENT_APPLY_CONFLICT"})
		return
	}
	response.OK(c, result)
}

// ApplySelected POST /api/v1/products/ai-text/batches/:id/apply-selected
func (h *Handler) ApplySelected(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的批次 ID")
		return
	}
	var body ApplySelectedBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	summary, err := h.Svc.ApplySelected(c, id, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, summary)
}

// RejectItem POST /api/v1/products/ai-text/items/:id/reject
func (h *Handler) RejectItem(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的子项 ID")
		return
	}
	if err := h.Svc.RejectItem(c.Request.Context(), id); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// UndoApplied POST /api/v1/products/ai-text/batches/:id/undo-applied
func (h *Handler) UndoApplied(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的批次 ID")
		return
	}
	summary, err := h.Svc.UndoApplied(c, id, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, summary)
}

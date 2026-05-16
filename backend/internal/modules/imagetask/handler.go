package imagetask

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Handler serves image task HTTP API.
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

type createBody struct {
	TaskType       string          `json:"taskType" binding:"required"`
	Provider       string          `json:"provider"`
	ProductID      string          `json:"productId"`
	SourceImageID  string          `json:"sourceImageId"`
	SourceImageURL string          `json:"sourceImageUrl"`
	Input          json.RawMessage `json:"input"`
}

// Create POST /api/v1/image/tasks
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	var body createBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	tt := strings.TrimSpace(body.TaskType)
	if !isValidTaskType(tt) {
		response.Fail(c, 400, response.CodeBadRequest, "invalid taskType")
		return
	}
	if strings.TrimSpace(body.SourceImageID) == "" && strings.TrimSpace(body.SourceImageURL) == "" {
		response.Fail(c, 400, response.CodeBadRequest, "sourceImageId or sourceImageUrl required")
		return
	}
	var productID *uuid.UUID
	if raw := strings.TrimSpace(body.ProductID); raw != "" {
		pid, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid productId")
			return
		}
		var n int64
		if err := h.Svc.DB.WithContext(c.Request.Context()).Model(&product.Product{}).Where("id = ?", pid).Count(&n).Error; err != nil || n == 0 {
			response.Fail(c, 400, response.CodeBadRequest, "product not found")
			return
		}
		productID = &pid
	}
	var srcImgID *uuid.UUID
	if raw := strings.TrimSpace(body.SourceImageID); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid sourceImageId")
			return
		}
		srcImgID = &u
	}
	inBytes := body.Input
	if len(inBytes) == 0 {
		inBytes = []byte("{}")
	}
	if !json.Valid(inBytes) {
		response.Fail(c, 400, response.CodeBadRequest, "input must be valid JSON")
		return
	}

	row, err := h.Svc.CreateAndPersist(c.Request.Context(), CreatePayload{
		TaskType:       tt,
		Provider:       body.Provider,
		ProductID:      productID,
		SourceImageID:  srcImgID,
		SourceImageURL: strings.TrimSpace(body.SourceImageURL),
		Input:          datatypes.JSON(inBytes),
		CreatedBy:      adminUUID(c),
	})
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "image.task.create",
			Resource:   "image_task",
			ResourceID: row.ID.String(),
			Status:     "success",
			Message:    logMessage(row),
		})
	}

	ctx := c.Request.Context()
	if h.Svc.QueueEnabled {
		tid, _ := c.Get(ctxkey.TraceID)
		var rid string
		if s, ok := tid.(string); ok {
			rid = s
		}
		if err := h.Svc.enqueueTask(ctx, row.ID, row.TaskType, row.Provider, row.CreatedBy, rid); err != nil {
			h.Svc.deleteTask(ctx, row.ID)
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, err.Error())
			return
		}
	} else {
		if err := h.Svc.ProcessSyncAfterCreate(ctx, row.ID, c); err != nil {
			fresh, ferr := h.Svc.GetByID(ctx, row.ID)
			if ferr == nil && fresh != nil {
				response.OK(c, fresh)
				return
			}
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
	}

	fresh, err := h.Svc.GetByID(ctx, row.ID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, fresh)
}

func logMessage(row *ImageTask) string {
	return strings.TrimSpace("taskType=" + row.TaskType + " provider=" + row.Provider + " productId=" + ptrUUIDStr(row.ProductID))
}

// List GET /api/v1/image/tasks
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	q := ListQuery{
		Page:     atoiQP(c, "page", 1),
		PageSize: atoiQP(c, "pageSize", 20),
		TaskType: c.Query("taskType"),
		Status:   c.Query("status"),
		Provider: c.Query("provider"),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if pid, err := uuid.Parse(raw); err == nil {
			q.ProductID = &pid
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

type taskDetailDTO struct {
	ID             uuid.UUID       `json:"id"`
	TaskType       string          `json:"taskType"`
	Provider       string          `json:"provider"`
	Status         string          `json:"status"`
	ProductID      *uuid.UUID      `json:"productId,omitempty"`
	SourceImageID  *uuid.UUID      `json:"sourceImageId,omitempty"`
	SourceImageURL string          `json:"sourceImageUrl,omitempty"`
	Input          json.RawMessage `json:"input,omitempty"`
	Output         json.RawMessage `json:"output,omitempty"`
	ResultFileID   *uuid.UUID      `json:"resultFileId,omitempty"`
	ResultURL      string          `json:"resultUrl,omitempty"`
	ErrorMessage   string          `json:"errorMessage,omitempty"`
	CreatedBy      *uuid.UUID      `json:"createdBy,omitempty"`
	StartedAt      *time.Time      `json:"startedAt,omitempty"`
	FinishedAt     *time.Time      `json:"finishedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// Monitor GET /api/v1/image/tasks/monitor
func (h *Handler) Monitor(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	snap, err := h.Svc.BuildMonitorSnapshot(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, snap)
}

// Get GET /api/v1/image/tasks/:id
func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	row, err := h.Svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	dto := taskDetailDTO{
		ID:             row.ID,
		TaskType:       row.TaskType,
		Provider:       row.Provider,
		Status:         row.Status,
		ProductID:      row.ProductID,
		SourceImageID:  row.SourceImageID,
		SourceImageURL: row.SourceImageURL,
		ResultFileID:   row.ResultFileID,
		ResultURL:      row.ResultURL,
		ErrorMessage:   row.ErrorMessage,
		CreatedBy:      row.CreatedBy,
		StartedAt:      row.StartedAt,
		FinishedAt:     row.FinishedAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if len(row.Input) > 0 {
		dto.Input = json.RawMessage(row.Input)
	}
	if len(row.Output) > 0 {
		dto.Output = json.RawMessage(row.Output)
	}
	response.OK(c, dto)
}

// Retry POST /api/v1/image/tasks/:id/retry
func (h *Handler) Retry(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.RetryEnqueue(c, id); err != nil {
		if errors.Is(err, ErrImageQueueUnavailable) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, err.Error())
			return
		}
		if strings.Contains(err.Error(), "only failed") {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		if !h.Svc.QueueEnabled {
			fresh, ferr := h.Svc.GetByID(c.Request.Context(), id)
			if ferr == nil && fresh != nil {
				response.OK(c, fresh)
				return
			}
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	fresh, err := h.Svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, fresh)
}

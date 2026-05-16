package aitask

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves read-only AI task HTTP API.
type Handler struct {
	Svc *Service
}

// List GET /api/v1/ai/tasks
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai tasks unavailable")
		return
	}
	q := ListQuery{
		Page:       atoiQP(c, "page", 1),
		PageSize:   atoiQP(c, "pageSize", 20),
		TaskType:   c.Query("taskType"),
		Status:     c.Query("status"),
		Provider:   c.Query("provider"),
		Model:      c.Query("model"),
		PromptCode: c.Query("promptCode"),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if pid, err := uuid.Parse(raw); err == nil {
			q.ProductID = &pid
		}
	}
	if raw := strings.TrimSpace(c.Query("conversationId")); raw != "" {
		if cid, err := uuid.Parse(raw); err == nil {
			q.ConversationID = &cid
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

// taskDetailDTO is returned by GET /api/v1/ai/tasks/:id (sensitive JSON fields redacted).
type taskDetailDTO struct {
	ID             uuid.UUID       `json:"id"`
	TaskType       string          `json:"taskType"`
	Provider       string          `json:"provider"`
	Model          string          `json:"model"`
	PromptCode     string          `json:"promptCode"`
	Input          json.RawMessage `json:"input,omitempty"`
	Output         json.RawMessage `json:"output,omitempty"`
	RawResponse    json.RawMessage `json:"rawResponse,omitempty"`
	Status         string          `json:"status"`
	ErrorMessage   string          `json:"errorMessage,omitempty"`
	TokenInput     int             `json:"tokenInput"`
	TokenOutput    int             `json:"tokenOutput"`
	CostAmount     float64         `json:"costAmount"`
	ProductID      *uuid.UUID      `json:"productId,omitempty"`
	ConversationID *uuid.UUID      `json:"conversationId,omitempty"`
	CreatedBy      *uuid.UUID      `json:"createdBy,omitempty"`
	StartedAt      *time.Time      `json:"startedAt,omitempty"`
	FinishedAt     *time.Time      `json:"finishedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// Get GET /api/v1/ai/tasks/:id
func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai tasks unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	row, err := h.Svc.GetByID(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}

	inRed := redactSensitiveJSON(row.Input)
	outRed := redactSensitiveJSON(row.Output)
	rawRed := redactSensitiveJSON(row.RawResponse)

	response.OK(c, taskDetailDTO{
		ID:             row.ID,
		TaskType:       row.TaskType,
		Provider:       row.Provider,
		Model:          row.Model,
		PromptCode:     row.PromptCode,
		Input:          inRed,
		Output:         outRed,
		RawResponse:    rawRed,
		Status:         row.Status,
		ErrorMessage:   row.ErrorMessage,
		TokenInput:     row.TokenInput,
		TokenOutput:    row.TokenOutput,
		CostAmount:     row.CostAmount,
		ProductID:      row.ProductID,
		ConversationID: row.ConversationID,
		CreatedBy:      row.CreatedBy,
		StartedAt:      row.StartedAt,
		FinishedAt:     row.FinishedAt,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	})
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

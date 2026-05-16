package customerchat

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler exposes customer chat HTTP API.
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

func atoiQ(c *gin.Context, key string, def int) int {
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

// ListConversations GET /api/v1/customer/conversations
func (h *Handler) ListConversations(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	q := ListQuery{
		Page:         atoiQ(c, "page", 1),
		PageSize:     atoiQ(c, "pageSize", 20),
		Platform:     c.Query("platform"),
		Status:       c.Query("status"),
		CustomerName: c.Query("customerName"),
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

// CreateConversation POST /api/v1/customer/conversations
func (h *Handler) CreateConversation(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	var body CreateConversationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreateConversation(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// GetConversation GET /api/v1/customer/conversations/:id
func (h *Handler) GetConversation(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.GetConversation(c, id)
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

// UpdateConversation PUT /api/v1/customer/conversations/:id
func (h *Handler) UpdateConversation(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body UpdateConversationBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.UpdateConversation(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// DeleteConversation DELETE /api/v1/customer/conversations/:id
func (h *Handler) DeleteConversation(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.DeleteConversation(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// ListMessages GET /api/v1/customer/conversations/:id/messages
func (h *Handler) ListMessages(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	cid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	rows, err := h.Svc.ListMessages(c, cid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// CreateMessage POST /api/v1/customer/conversations/:id/messages
func (h *Handler) CreateMessage(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	cid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body CreateMessageBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreateMessage(c, cid, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// MarkReplied POST /api/v1/customer/conversations/:id/mark-replied
func (h *Handler) MarkReplied(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	cid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body MarkRepliedBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.MarkReplied(c, cid, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// GenerateReply POST /api/v1/customer/conversations/:id/ai/generate-reply
func (h *Handler) GenerateReply(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	cid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body GenerateReplyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.GenerateReply(c, cid, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// UpdateSuggestion PUT /api/v1/customer/reply-suggestions/:id
func (h *Handler) UpdateSuggestion(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body UpdateSuggestionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if err := h.Svc.UpdateSuggestion(c, id, body, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// AcceptSuggestion POST /api/v1/customer/reply-suggestions/:id/accept
func (h *Handler) AcceptSuggestion(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body AcceptSuggestionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if err := h.Svc.AcceptSuggestion(c, id, body, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// DiscardSuggestion POST /api/v1/customer/reply-suggestions/:id/discard
func (h *Handler) DiscardSuggestion(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.DiscardSuggestion(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

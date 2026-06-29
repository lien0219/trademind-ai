package customerchat

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
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
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ShopID = &u
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
	q.Keyword = c.Query("keyword")
	q.PendingReply = parseBoolQuery(c.Query("pendingReply"))
	q.HasAiSuggestion = parseBoolQuery(c.Query("hasAiSuggestion"))
	q.SendFailed = parseBoolQuery(c.Query("sendFailed"))
	q.HasOrder = parseBoolQuery(c.Query("hasOrder"))
	if raw := strings.TrimSpace(c.Query("updatedStart")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.UpdatedStart = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("updatedEnd")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			q.UpdatedEnd = &t
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
	if out != nil {
		out.CanWrite = adminperm.CanWriteCustomer(c, h.Svc.DB)
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
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可生成 AI 建议")
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

// SendPlatformMessage POST /api/v1/customer/conversations/:id/send-platform-message
func (h *Handler) SendPlatformMessage(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可发送客服消息")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body SendPlatformMessageBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.SendPlatformMessage(c, id, body, adminUUID(c))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, 404, response.CodeNotFound, "not found")
		case errors.Is(err, platformp.ErrCustomerMessageNotImplemented):
			response.Fail(c, http.StatusNotImplemented, response.CodeBadRequest, err.Error())
		case errors.Is(err, platformp.ErrPlatformCustomerMessagePermissionDenied):
			response.Fail(c, 403, response.CodeBadRequest, "平台客服权限不足，请确认已在对应电商平台开放后台申请客服消息权限并重新授权店铺。（Amazon：Seller Central / SP-API 申请 Buyer-Seller Messaging / Messaging API 权限）")
		default:
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
		}
		return
	}
	response.OK(c, out)
}

// GetDashboard GET /api/v1/customer/dashboard
func (h *Handler) GetDashboard(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	out, err := h.Svc.GetDashboard(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// ListSuggestions GET /api/v1/customer/conversations/:id/ai-suggestions
func (h *Handler) ListSuggestions(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	cid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	rows, err := h.Svc.ListSuggestions(c, cid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// GenerateAISuggestion POST alias for generate-reply
func (h *Handler) GenerateAISuggestion(c *gin.Context) {
	h.GenerateReply(c)
}

// ApplySuggestion POST /api/v1/customer/ai-suggestions/:id/apply
func (h *Handler) ApplySuggestion(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可编辑 AI 建议")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body ApplySuggestionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if err := h.Svc.ApplySuggestion(c, id, body, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// RejectSuggestion POST /api/v1/customer/ai-suggestions/:id/reject
func (h *Handler) RejectSuggestion(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可拒绝 AI 建议")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body RejectSuggestionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if err := h.Svc.RejectSuggestion(c, id, body, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

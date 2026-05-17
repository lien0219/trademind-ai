package productpublish

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

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

// Register mounts publish-related routes (call after product routes is fine; paths are distinct).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/products/:id/publish", h.Publish)
	g.GET("/products/:id/publications", h.ListByProduct)
	g.GET("/product-publish/tasks", h.ListTasks)
	g.GET("/product-publish/tasks/:id", h.GetTask)
	g.POST("/product-publish/tasks/:id/retry", h.RetryTask)
}

func (h *Handler) Publish(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "product publish unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body PublishRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreatePublishTask(c, pid, body, adminUUID(c))
	if err != nil {
		switch {
		case errors.Is(err, platformp.ErrPlatformProductPublishPermissionDenied):
			response.Fail(c, http.StatusForbidden, response.CodeBadRequest, err.Error()+" — 请确认已在对应开放平台（如 TikTok Shop Partner Center / Shopee Open Platform / Lazada Open Platform）申请商品刊登相关权限并重新授权。")
			return
		case errors.Is(err, platformp.ErrProductPublishNotImplemented):
			response.Fail(c, http.StatusNotImplemented, response.CodeBadRequest, err.Error())
			return
		case errors.Is(err, platformp.ErrManualProductPublishUnsupported):
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		default:
			msg := err.Error()
			if strings.Contains(msg, "platform config incomplete") || strings.Contains(msg, "platform publish config incomplete") ||
				strings.Contains(msg, "shop is not authorized") {
				response.Fail(c, 400, response.CodeBadRequest, msg)
				return
			}
			response.Fail(c, 400, response.CodeBadRequest, msg)
			return
		}
	}
	response.OK(c, out)
}

func (h *Handler) ListByProduct(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "product publish unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	list, err := h.Svc.ListPublicationsByProduct(c.Request.Context(), pid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

func (h *Handler) ListTasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "product publish unavailable")
		return
	}
	q := ListTasksQuery{
		Page:     atoiQ(c, "page", 1),
		PageSize: atoiQ(c, "pageSize", 20),
		Platform: c.Query("platform"),
		Status:   c.Query("status"),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		if u, err := uuid.Parse(raw); err == nil {
			q.ProductID = &u
		}
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
	res, err := h.Svc.ListTasks(c.Request.Context(), q)
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

func (h *Handler) GetTask(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "product publish unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.GetDTO(c.Request.Context(), id)
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

func (h *Handler) RetryTask(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "product publish unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.RetryFailed(c, id, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

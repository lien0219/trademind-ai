package product

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler exposes product draft HTTP API.
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

func queryTruthy(c *gin.Context, key string) bool {
	v := strings.TrimSpace(strings.ToLower(c.Query(key)))
	return v == "1" || v == "true" || v == "yes"
}

// List GET /api/v1/products
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	q := ListQuery{
		Page:                 atoiQP(c, "page", 1),
		PageSize:             atoiQP(c, "pageSize", 20),
		Status:               c.Query("status"),
		Source:               c.Query("source"),
		Keyword:              c.Query("keyword"),
		MissingAiTitle:       queryTruthy(c, "missingAiTitle"),
		MissingAiDescription: queryTruthy(c, "missingAiDescription"),
		ReadinessBlocked:     strings.TrimSpace(strings.ToLower(c.Query("readiness"))) == "blocked",
		Publishable:          queryTruthy(c, "publishable"),
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

// Create POST /api/v1/products
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.Create(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// Get GET /api/v1/products/:id
func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.Get(c, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// Put PUT /api/v1/products/:id
func (h *Handler) Put(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body UpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.Update(c, id, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// Delete DELETE /api/v1/products/:id
func (h *Handler) Delete(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.Delete(c, id, adminUUID(c)); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// OptimizeTitle POST /api/v1/products/:id/ai/optimize-title
func (h *Handler) OptimizeTitle(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body OptimizeTitleBody
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
			return
		}
	}
	out, err := h.Svc.OptimizeTitle(c, id, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ApplyAITitle POST /api/v1/products/:id/apply-ai-title
func (h *Handler) ApplyAITitle(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body ApplyAITitleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.ApplyAITitle(c, id, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// GenerateDescription POST /api/v1/products/:id/ai/generate-description
func (h *Handler) GenerateDescription(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body GenerateDescriptionBody
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
			return
		}
	}
	out, err := h.Svc.GenerateDescription(c, id, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ApplyAIDescription POST /api/v1/products/:id/apply-ai-description
func (h *Handler) ApplyAIDescription(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body ApplyAIDescriptionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.ApplyAIDescription(c, id, body, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ListAITasks GET /api/v1/products/:id/ai/tasks
func (h *Handler) ListAITasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	items, err := h.Svc.ListRecentAITasks(c, id, 15)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": items})
}

// SearchSKUs GET /api/v1/product-skus/search
func (h *Handler) SearchSKUs(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	q := SearchSKUsQuery{
		Keyword: c.Query("keyword"),
		Limit:   atoiQP(c, "limit", 20),
	}
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		q.ProductID = &raw
	}
	list, err := h.Svc.SearchSKUs(c, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": list})
}

package operationlog

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves operation log HTTP API.
type Handler struct {
	Svc *Service
	DB  *gorm.DB
}

// List GET /api/v1/operation-logs
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "operation logs unavailable")
		return
	}
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermOperationLogView) {
		return
	}
	q := ListQuery{
		Page:     atoiQP(c, "page", 1),
		PageSize: atoiQP(c, "pageSize", 20),
		Action:   c.Query("action"),
		Username: c.Query("username"),
		Resource: c.Query("resource"),
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

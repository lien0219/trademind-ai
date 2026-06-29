package adminuser

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves admin user management HTTP API.
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

func (h *Handler) requireManage(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "用户管理不可用")
		return false
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermUserManage) {
		return false
	}
	return true
}

// List GET /api/v1/admin/users
func (h *Handler) List(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	q := ListQuery{
		Page:     atoiQP(c, "page", 1),
		PageSize: atoiQP(c, "pageSize", 20),
		Role:     c.Query("role"),
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
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

// Get GET /api/v1/admin/users/:id
func (h *Handler) Get(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的用户 ID")
		return
	}
	row, err := h.Svc.Get(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "用户不存在")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, row)
}

// Create POST /api/v1/admin/users
func (h *Handler) Create(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	row, err := h.Svc.Create(c, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrDuplicateAccount) {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// Update PATCH /api/v1/admin/users/:id
func (h *Handler) Update(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的用户 ID")
		return
	}
	var body UpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	row, err := h.Svc.Update(c, id, body, adminUUID(c))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, 404, response.CodeNotFound, "用户不存在")
		case errors.Is(err, ErrSelfDisable), errors.Is(err, ErrSelfRoleDowngrade):
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
		default:
			response.HandleError(c, err)
		}
		return
	}
	response.OK(c, row)
}

// SetStorePermissions PUT /api/v1/admin/users/:id/store-permissions
func (h *Handler) SetStorePermissions(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的用户 ID")
		return
	}
	var body SetStorePermissionsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	row, err := h.Svc.SetStorePermissions(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "用户不存在")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
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

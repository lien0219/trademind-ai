package tenantmgmt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/passwordpolicy"
	"gorm.io/gorm"
)

// Handler is a control-plane endpoint. It never binds tenant scope to an
// arbitrary request header and does not bypass application RBAC.
type Handler struct {
	Service *Service
}

func (h *Handler) actor(c *gin.Context) (uuid.UUID, bool) {
	if h == nil || h.Service == nil || h.Service.DB == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeInternalError, "tenant service unavailable")
		return uuid.Nil, false
	}
	raw, ok := c.Get(ctxkey.AdminID)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return uuid.Nil, false
	}
	idStr, castOK := raw.(string)
	if !castOK {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "platform operator required")
		return uuid.Nil, false
	}
	actor, err := uuid.Parse(strings.TrimSpace(idStr))
	if err != nil || actor == uuid.Nil || h.Service.requireOperator(c.Request.Context(), actor) != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "platform operator required")
		return uuid.Nil, false
	}
	return actor, true
}

func (h *Handler) List(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	rows, total, err := h.Service.ListTenants(c.Request.Context(), c.Query("keyword"), c.Query("status"), page, pageSize)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) Create(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var in CreateTenantInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant request")
		return
	}
	t, err := h.Service.Create(c.Request.Context(), actor, in)
	if err != nil {
		// Only deliberate validation and conflict messages are public. Never
		// echo raw SQL, credential material, or unexpected storage errors.
		// Trace the underlying failure in the server logs for diagnostics.
		switch err.Error() {
		case "invalid tenant metadata":
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "企业标识或资料不符合要求，请检查企业名称和标识")
		case "tenant owner email is required":
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请输入有效的企业管理员邮箱")
		case passwordpolicy.Hint:
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "初始密码不符合要求："+passwordpolicy.Hint)
		case "invalid idempotency key":
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "创建请求标识无效，请刷新后重试")
		case "idempotency key reused with different request":
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, "创建请求内容发生变化，请关闭窗口后重新创建")
		case "tenant owner account is inactive":
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, "管理员邮箱已关联停用账号，请先处理账号状态")
		case "platform operator required":
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "没有创建企业的权限")
		default:
			slog.ErrorContext(c.Request.Context(), "tenant creation failed", "error", err)
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "创建企业失败，请联系管理员使用 traceId 查询后端日志")
		}
		return
	}
	response.OK(c, t)
}

func (h *Handler) Update(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant id")
		return
	}
	var in TenantUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant request")
		return
	}
	row, err := h.Service.UpdateTenant(c.Request.Context(), actor, id, in)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to update tenant")
		return
	}
	response.OK(c, row)
}

func (h *Handler) Detail(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant id")
		return
	}
	row, err := h.Service.TenantDetail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "tenant not found")
		return
	}
	response.OK(c, row)
}

func (h *Handler) Current(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	if _, err := h.Service.requireTenantMember(c.Request.Context(), tid, uid); err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant access required")
		return
	}
	row, err := h.Service.CurrentTenant(c.Request.Context(), tid)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "tenant not found")
		return
	}
	response.OK(c, row)
}

func (h *Handler) CurrentUpdate(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	if _, err := h.Service.requireTenantAdmin(c.Request.Context(), tid, uid); err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant administrator required")
		return
	}
	var in TenantUpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant request")
		return
	}
	row, err := h.Service.UpdateTenantAsMember(c.Request.Context(), uid, tid, in)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to update tenant")
		return
	}
	response.OK(c, row)
}

func (h *Handler) Members(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	if _, err := h.Service.requireTenantPermission(c.Request.Context(), tid, uid, adminperm.PermUserManage); err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant user management required")
		return
	}
	rows, err := h.Service.Members(c.Request.Context(), tid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

func (h *Handler) UpdateMember(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	mid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid member id")
		return
	}
	var in struct {
		Role             string                      `json:"role"`
		Status           string                      `json:"status"`
		StorePermissions []admin.UserStorePermission `json:"storePermissions"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid member request")
		return
	}
	if err := h.Service.UpdateMember(c.Request.Context(), uid, tid, mid, in.Role, in.Status, in.StorePermissions); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to update member")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) Leave(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	if err := h.Service.Leave(c.Request.Context(), uid, tid); err != nil {
		if errors.Is(err, ErrTenantLeaveForbidden) {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "Root 和企业管理员不能退出当前企业")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to leave tenant")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) Invitations(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	if _, err := h.Service.requireTenantPermission(c.Request.Context(), tid, uid, adminperm.PermUserManage); err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant user management required")
		return
	}
	rows, err := h.Service.Invitations(c.Request.Context(), tid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

func (h *Handler) Invite(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	var in InvitationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid invitation request")
		return
	}
	row, token, err := h.Service.CreateInvitation(c.Request.Context(), uid, tid, in)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to create invitation")
		return
	}
	response.OK(c, gin.H{"invitation": row, "token": token})
}

func (h *Handler) CancelInvitation(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid <= 0 {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	iid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid invitation id")
		return
	}
	if err := h.Service.CancelInvitation(c.Request.Context(), uid, tid, iid); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to cancel invitation")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) AcceptInvitation(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid invitation request")
		return
	}
	var actor admin.AdminUser
	if err := h.Service.DB.WithContext(c.Request.Context()).Select("email").First(&actor, "id = ?", uid).Error; err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
		return
	}
	tid, err := h.Service.AcceptInvitation(c.Request.Context(), uid, in.Token, actor.Email)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to accept invitation")
		return
	}
	response.OK(c, gin.H{"tenantId": tid})
}

func (h *Handler) Audit(c *gin.Context) {
	if _, ok := h.actor(c); !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant id")
		return
	}
	var rows []operationlog.OperationLog
	if err := h.Service.DB.WithContext(c.Request.Context()).Where("tenant_id = ?", id).Order("created_at DESC").Limit(200).Find(&rows).Error; err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

func currentUser(c *gin.Context) (uuid.UUID, bool) {
	raw, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(fmt.Sprint(raw)))
	return id, err == nil && id != uuid.Nil
}

func (s *Service) requireTenantMember(ctx context.Context, tenantID int64, userID uuid.UUID) (*admin.TenantMembership, error) {
	var m admin.TenantMembership
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, admin.MembershipActive).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (h *Handler) SetStatus(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant id")
		return
	}
	var in struct {
		Status  string `json:"status" binding:"required"`
		Version int64  `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant status")
		return
	}
	row, err := h.Service.SetStatus(c.Request.Context(), actor, id, in.Status, in.Version)
	if err != nil {
		if errors.Is(err, ErrTenantVersionConflict) {
			response.Fail(c, http.StatusConflict, response.CodeConflict, "tenant version conflict")
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "tenant not found")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "unable to update tenant")
		return
	}
	response.OK(c, row)
}

// Delete logically removes a tenant and preserves its business history.
func (h *Handler) Delete(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tenant id")
		return
	}
	var in struct {
		Code    string `json:"code" binding:"required"`
		Version int64  `json:"version" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "tenant code and version required")
		return
	}
	if err := h.Service.DeleteTenant(c.Request.Context(), actor, id, in.Code, in.Version); err != nil {
		switch {
		case errors.Is(err, ErrTenantVersionConflict):
			response.Fail(c, http.StatusConflict, response.CodeConflict, "tenant version conflict")
		case errors.Is(err, ErrTenantMustBeInactive):
			response.Fail(c, http.StatusConflict, response.CodeConflict, "suspend tenant before deleting")
		case errors.Is(err, ErrTenantCodeMismatch):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "tenant code does not match")
		case errors.Is(err, ErrTenantAlreadyDeleted):
			response.Fail(c, http.StatusConflict, response.CodeConflict, "tenant already deleted")
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "tenant not found")
		default:
			response.HandleError(c, err)
		}
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	r := g.Group("/platform/tenants")
	r.GET("", h.List)
	r.POST("", h.Create)
	r.GET("/:id", h.Detail)
	r.PATCH("/:id", h.Update)
	r.PUT("/:id", h.Update)
	r.DELETE("/:id", h.Delete)
	r.PATCH("/:id/status", h.SetStatus)
	r.GET("/:id/audit", h.Audit)
	current := g.Group("/tenant/current")
	current.GET("", h.Current)
	current.PATCH("", h.CurrentUpdate)
	current.GET("/members", h.Members)
	current.PATCH("/members/:id", h.UpdateMember)
	current.POST("/members/leave", h.Leave)
	current.GET("/invitations", h.Invitations)
	current.POST("/invitations", h.Invite)
	current.DELETE("/invitations/:id", h.CancelInvitation)
	// Canonical tenant-member and invitation paths are rooted at /tenant;
	// retain the /tenant/current aliases for existing clients during rollout.
	tenant := g.Group("/tenant")
	tenant.GET("/members", h.Members)
	tenant.PATCH("/members/:id", h.UpdateMember)
	tenant.POST("/members/leave", h.Leave)
	tenant.GET("/invitations", h.Invitations)
	tenant.POST("/invitations", h.Invite)
	tenant.DELETE("/invitations/:id", h.CancelInvitation)
	g.POST("/tenant/invitations/accept", h.AcceptInvitation)
}

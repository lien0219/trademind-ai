package taskcenter

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

var (
	errTaskPermissionDenied        = errors.New("task center permission denied")
	errTaskTenantMissing           = errors.New("task center tenant missing")
	errTaskInvalidRequest          = errors.New("invalid task center request")
	errTaskConflict                = errors.New("task center state conflict")
	errTaskDependencyUnavailable   = errors.New("task center dependency unavailable")
	errTaskAuditOutcomeUnavailable = errors.New("task center outcome audit unavailable")
)

func (h *Handler) requirePermission(permission string) gin.HandlerFunc {
	return h.requireAnyPermission(permission)
}

func (h *Handler) requireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "task center unavailable")
			c.Abort()
			return
		}

		rawID, ok := c.Get(ctxkey.AdminID)
		idString, isString := rawID.(string)
		userID, parseErr := uuid.Parse(strings.TrimSpace(idString))
		if !ok || !isString || parseErr != nil || userID == uuid.Nil {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication required")
			c.Abort()
			return
		}

		tenantID, err := adminperm.TenantIDFromGin(c)
		if err != nil || tenantID < 0 {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
			c.Abort()
			return
		}

		principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
		if err != nil {
			response.HandleError(c, err)
			c.Abort()
			return
		}
		if principal == nil || principal.UserID != userID || principal.Disabled || !knownTaskCenterRole(principal.Role) {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "permission denied")
			c.Abort()
			return
		}
		allowed := false
		for _, permission := range permissions {
			if strings.TrimSpace(permission) != "" && principal.Can(permission) {
				allowed = true
				break
			}
		}
		if !allowed {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "permission denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func requestTaskPrincipal(c *gin.Context, db *gorm.DB, requiredPermissions ...string) (int64, *adminperm.Principal, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID < 0 {
		return 0, nil, errTaskTenantMissing
	}
	principal, err := adminperm.LoadPrincipal(c, db)
	if err != nil {
		return 0, nil, err
	}
	if principal == nil || principal.Disabled || !knownTaskCenterRole(principal.Role) {
		return 0, nil, errTaskPermissionDenied
	}
	if len(requiredPermissions) == 0 {
		requiredPermissions = []string{adminperm.PermTaskRetry}
	}
	for _, permission := range requiredPermissions {
		if principal.Can(permission) {
			return tenantID, principal, nil
		}
	}
	return 0, nil, errTaskPermissionDenied
}

func (h *Handler) setFailureScope(c *gin.Context, p *ListFailureParams) bool {
	tenantID, principal, err := requestTaskPrincipal(c, h.Svc.DB, adminperm.PermTaskRetry)
	if err != nil {
		if errors.Is(err, errTaskPermissionDenied) {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "permission denied")
		} else if errors.Is(err, errTaskTenantMissing) {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		} else {
			response.HandleError(c, err)
		}
		return false
	}
	p.TenantID = tenantID
	p.TenantScopeSet = true
	p.RestrictShopScope = !principal.IsAdmin()
	p.AllowedShopIDs = principal.AllowedStoreIDs()
	return true
}

func knownTaskCenterRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case adminperm.RoleAdmin, adminperm.RoleOperator, adminperm.RoleReadonly, adminperm.RoleReviewer:
		return true
	default:
		return adminperm.IsCustomRole(role)
	}
}

func (s *Service) ensureTaskStoreScope(c *gin.Context, task UnifiedTaskDTO, write bool) error {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID < 0 || task.TenantID != tenantID {
		return gorm.ErrRecordNotFound
	}
	principal, err := adminperm.LoadPrincipal(c, s.DB)
	if err != nil {
		return err
	}
	if principal == nil || principal.Disabled || !knownTaskCenterRole(principal.Role) || !principal.Can(adminperm.PermTaskRetry) {
		return errTaskPermissionDenied
	}
	shopIDText := strings.TrimSpace(task.ShopID)
	if shopIDText == "" {
		return nil
	}
	shopID, err := uuid.Parse(shopIDText)
	if err != nil || shopID == uuid.Nil {
		return gorm.ErrRecordNotFound
	}
	if !principal.IsAdmin() {
		if write && !principal.CanOperateStore(shopID) || !write && !principal.CanViewStore(shopID) {
			return gorm.ErrRecordNotFound
		}
	}
	var count int64
	if err := s.DB.WithContext(c.Request.Context()).Table("shops").Where("id = ? AND tenant_id = ?", shopID, tenantID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) ensureTaskAlertStoreScope(c *gin.Context, alert TaskAlert, write bool) error {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID != alert.TenantID {
		return gorm.ErrRecordNotFound
	}
	id, err := uuid.Parse(strings.TrimSpace(alert.SourceID))
	if err != nil || id == uuid.Nil {
		return gorm.ErrRecordNotFound
	}
	task, err := s.unifiedOne(c.Request.Context(), tenantID, alert.TaskType, id, time.Now().UTC())
	if err != nil {
		return gorm.ErrRecordNotFound
	}
	return s.ensureTaskStoreScope(c, task, write)
}

func taskScopeFailureResponse(c *gin.Context, err error) {
	taskFailureResponse(c, err)
}

func taskFailureResponse(c *gin.Context, err error) {
	if c == nil || err == nil {
		return
	}
	if errors.Is(err, errTaskAuditOutcomeUnavailable) {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "notification was attempted, but its result could not be fully recorded")
		return
	}
	if errors.Is(err, errTaskAuditUnavailable) {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "audit unavailable; operation was not completed")
		return
	}
	if errors.Is(err, errTaskPermissionDenied) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "permission denied")
		return
	}
	if errors.Is(err, errTaskTenantMissing) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context required")
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
		return
	}
	if errors.Is(err, errTaskInvalidRequest) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if errors.Is(err, errTaskConflict) {
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
		return
	}
	if errors.Is(err, errTaskDependencyUnavailable) {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "task center dependency unavailable")
		return
	}
	response.HandleError(c, err)
}

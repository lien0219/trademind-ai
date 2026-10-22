package adminuser

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RoleRow is an authoritative server-side role record with member counts.
type RoleRow struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	Enabled     bool     `json:"enabled"`
	System      bool     `json:"system"`
	MemberCount int64    `json:"memberCount"`
}

type roleInput struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Permissions []string `json:"permissions"`
	Enabled     *bool    `json:"enabled"`
}

// PermissionCatalog is server-authoritative; client-side buttons are UX only.
func (h *Handler) PermissionCatalog(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	response.OK(c, gin.H{"permissions": adminperm.DelegablePermissions()})
}

func (h *Handler) ListRoles(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	rows := make([]RoleRow, 0)
	for _, code := range []string{adminperm.RoleAdmin, adminperm.RoleOperator, adminperm.RoleReviewer, adminperm.RoleReadonly} {
		rows = append(rows, RoleRow{
			Code: code, Name: code, Permissions: adminperm.StrictPermissionsForRole(code),
			System: true, Enabled: true,
		})
	}
	var custom []admin.RoleDefinition
	if err := h.Svc.DB.WithContext(c.Request.Context()).Where("tenant_id = ?", tid).Order("created_at ASC").Find(&custom).Error; err != nil {
		response.HandleError(c, err)
		return
	}
	for _, role := range custom {
		var permissions []string
		if err := json.Unmarshal([]byte(role.PermissionsJSON), &permissions); err != nil {
			response.HandleError(c, err)
			return
		}
		validated, err := adminperm.ValidateCustomPermissions(permissions)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		rows = append(rows, RoleRow{
			Code: role.Code, Name: role.Name, Description: role.Description,
			Permissions: validated, Enabled: role.Enabled,
		})
	}
	for i := range rows {
		if err := h.Svc.DB.WithContext(c.Request.Context()).Model(&admin.AdminUser{}).
			Where("tenant_id = ? AND role = ?", tid, rows[i].Code).
			Count(&rows[i].MemberCount).Error; err != nil {
			response.HandleError(c, err)
			return
		}
	}
	response.OK(c, gin.H{"list": rows})
}

func (h *Handler) CreateRole(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	var body roleInput
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "角色名称必填")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	name, err := validateRoleName(*body.Name)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	permissions, err := adminperm.ValidateCustomPermissions(body.Permissions)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	description := ""
	if body.Description != nil {
		description = strings.TrimSpace(*body.Description)
	}
	if len([]rune(description)) > 400 {
		response.Fail(c, 400, response.CodeBadRequest, "角色描述长度超过限制")
		return
	}
	encoded, err := json.Marshal(permissions)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	code := "r_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:24]
	row := admin.RoleDefinition{
		TenantID: tid, Code: code, Name: name, Description: description,
		PermissionsJSON: string(encoded), Enabled: true, CreatedBy: adminUUID(c),
	}
	if err := h.Svc.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		response.HandleError(c, err)
		return
	}
	h.recordRoleAudit(c, "role.create", row.Code, fmt.Sprintf("name=%s granted=%d", row.Name, len(permissions)))
	response.OK(c, RoleRow{
		Code: row.Code, Name: row.Name, Description: row.Description,
		Permissions: permissions, Enabled: true, MemberCount: 0,
	})
}

func validateRoleName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > 80 {
		return "", fmt.Errorf("角色名称为必填项且不能超过80字")
	}
	return name, nil
}

// UpdateRole changes permissions atomically with member security-version bumps.
// All nodes read token_version from SQL, so revoked permissions cannot remain
// usable via the process-local principal cache.
func (h *Handler) UpdateRole(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	code := strings.TrimSpace(c.Param("code"))
	if !adminperm.IsCustomRole(code) {
		response.Fail(c, 400, response.CodeBadRequest, "仅允许修改自定义角色")
		return
	}
	var body roleInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	var changed admin.RoleDefinition
	err = h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND code = ?", tid, code).First(&changed).Error; err != nil {
			return err
		}
		if body.Name != nil {
			name, err := validateRoleName(*body.Name)
			if err != nil {
				return err
			}
			changed.Name = name
		}
		if body.Description != nil {
			changed.Description = strings.TrimSpace(*body.Description)
			if len([]rune(changed.Description)) > 400 {
				return fmt.Errorf("角色描述长度超过限制")
			}
		}
		// nil means no change; explicitly empty slices clear all grants.
		if body.Permissions != nil {
			grants, err := adminperm.ValidateCustomPermissions(body.Permissions)
			if err != nil {
				return err
			}
			encoded, err := json.Marshal(grants)
			if err != nil {
				return err
			}
			changed.PermissionsJSON = string(encoded)
		}
		if body.Enabled != nil {
			changed.Enabled = *body.Enabled
		}
		if err := tx.Save(&changed).Error; err != nil {
			return err
		}
		return tx.Model(&admin.AdminUser{}).
			Where("tenant_id = ? AND role = ?", tid, code).
			UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "角色不存在")
		} else if strings.Contains(err.Error(), "角色") || strings.Contains(err.Error(), "权限") {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
		} else {
			response.HandleError(c, err)
		}
		return
	}
	// Invalidate local entries too; other nodes observe the SQL version bump.
	var ids []uuid.UUID
	if err := h.Svc.DB.WithContext(c.Request.Context()).Model(&admin.AdminUser{}).
		Where("tenant_id = ? AND role = ?", tid, code).Pluck("id", &ids).Error; err == nil {
		for _, id := range ids {
			adminperm.InvalidateUserPermissionCache(id)
		}
	}
	var grants []string
	if err := json.Unmarshal([]byte(changed.PermissionsJSON), &grants); err != nil {
		response.HandleError(c, err)
		return
	}
	sort.Strings(grants)
	h.recordRoleAudit(c, "role.update", code, fmt.Sprintf("name=%s enabled=%t granted=%d members=%d", changed.Name, changed.Enabled, len(grants), len(ids)))
	response.OK(c, RoleRow{
		Code: changed.Code, Name: changed.Name, Description: changed.Description,
		Enabled: changed.Enabled, Permissions: grants, MemberCount: int64(len(ids)),
	})
}

func (h *Handler) recordRoleAudit(c *gin.Context, action, code, detail string) {
	if h == nil || h.Svc == nil || h.Svc.OpLog == nil {
		return
	}
	_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminUUID(c), Action: action, Resource: "admin_role",
		ResourceID: code, Status: "success", Message: detail,
	})
}

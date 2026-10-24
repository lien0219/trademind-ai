package adminuser

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrRoleHasMembers = errors.New("此角色仍有关联用户，请先调整成员角色后再删除")

// Delete soft-deletes the account without erasing historical business or audit
// records. The disabled state, SQL security version, deleted marker and audit
// commit together. Every access/refresh path re-reads this persisted identity.
func (s *Service) Delete(c *gin.Context, userID uuid.UUID, actorID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return errors.New("adminuser: no db")
	}
	if actorID == nil || *actorID == uuid.Nil {
		return inputError("无法确认当前登录账号")
	}
	if *actorID == userID {
		return inputError("不能删除当前登录账号")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := lockTenantAdminMutations(tx, tid); err != nil {
			return err
		}
		var user admin.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ? AND tenant_id = ?", userID, tid).Error; err != nil {
			return err
		}
		if normalizeRole(user.Role) == adminperm.RoleAdmin && strings.EqualFold(strings.TrimSpace(user.Status), admin.StatusActive) {
			var others int64
			if err := tx.Model(&admin.AdminUser{}).Where("tenant_id = ? AND LOWER(TRIM(role)) = ? AND LOWER(TRIM(status)) = ? AND id <> ?", tid, adminperm.RoleAdmin, admin.StatusActive, userID).Count(&others).Error; err != nil {
				return err
			}
			if others == 0 {
				return inputError("必须保留至少一名启用的管理员")
			}
		}
		if err := tx.Model(&user).Updates(map[string]any{"status": admin.StatusDisabled, "token_version": gorm.Expr("token_version + 1")}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&admin.UserStorePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&user).Error; err != nil {
			return err
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "user.delete", "admin_user", userID.String(), "account soft-deleted; prior credentials and store grants invalidated; historical records retained")
	})
	if err == nil {
		adminperm.InvalidateUserPermissionCache(userID)
	}
	return err
}

// DeleteRole shares the assignment lock with user creation/role updates, so a
// role cannot be removed between its member check and a concurrent assignment.
func (s *Service) DeleteRole(c *gin.Context, code string, actorID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return errors.New("adminuser: no db")
	}
	if actorID == nil || *actorID == uuid.Nil {
		return inputError("无法确认当前登录账号")
	}
	if !adminperm.IsCustomRole(code) {
		return inputError("系统角色不能删除")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := lockTenantAdminMutations(tx, tid); err != nil {
			return err
		}
		var role admin.RoleDefinition
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND code = ?", tid, code).First(&role).Error; err != nil {
			return err
		}
		var members int64
		if err := tx.Model(&admin.AdminUser{}).Where("tenant_id = ? AND role = ?", tid, code).Count(&members).Error; err != nil {
			return err
		}
		if members != 0 {
			return ErrRoleHasMembers
		}
		var grants []string
		if err := json.Unmarshal([]byte(role.PermissionsJSON), &grants); err != nil {
			return err
		}
		before, err := json.Marshal(roleAuditState{Name: role.Name, Description: role.Description, Permissions: grants, Enabled: role.Enabled})
		if err != nil {
			return err
		}
		if err := tx.Delete(&role).Error; err != nil {
			return err
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "role.delete", "admin_role", code, string(before))
	})
}

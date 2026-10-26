package securitymod

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Rotation touches secrets across all tenants. The persisted singleton Root
// may operate from an active, explicit business session; a normal tenant
// administrator cannot escalate into global key rotation.
func (h *Handler) requireGlobalKeyAdministrator(c *gin.Context) bool {
	if h == nil || h.DB == nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "仅全局系统管理员可管理密钥轮换")
		return false
	}
	p, err := adminperm.ResolvePlatformPrincipal(c, h.DB)
	if err != nil || p == nil || !p.IsAdmin() || !p.Can(adminperm.PermSecurityKeyRotate) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "仅全局系统管理员可管理密钥轮换")
		return false
	}
	return true
}

// Recheck and lock the persisted actor in the mutation transaction. A failed
// audit append rolls back the rotation change; no secret material is logged.
func (h *Handler) withRotationAudit(c *gin.Context, action string, mutate func(*Service) (string, error)) error {
	if h == nil || h.Svc == nil || h.Svc.DB == nil || h.Svc.OpLogs == nil || h.Svc.OpLogs.DB == nil {
		return fmt.Errorf("key rotation audit unavailable")
	}
	uid, err := uuid.Parse(c.GetString(ctxkey.AdminID))
	if err != nil || uid == uuid.Nil {
		return fmt.Errorf("key rotation actor unavailable")
	}
	return h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// Serialize global rotation controls across application instances.
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(0x544d4b4559524f54)).Error; err != nil {
				return err
			}
		}
		var actor admin.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&actor, "id = ?", uid).Error; err != nil {
			return err
		}
		if actor.TenantID != 0 || !strings.EqualFold(strings.TrimSpace(actor.Role), adminperm.RoleAdmin) || !strings.EqualFold(strings.TrimSpace(actor.Status), admin.StatusActive) {
			return fmt.Errorf("key rotation actor is no longer authorized")
		}
		// Recheck the Root singleton under the transaction, not merely at the
		// request middleware. Tenant-zero legacy administrators retain the
		// existing control-plane path; positive tenant sessions require Root.
		tid, err := adminperm.TenantIDFromGin(c)
		if err != nil {
			return fmt.Errorf("rotation tenant context unavailable: %w", err)
		}
		if tid > 0 {
			root, err := adminperm.IsRoot(c.Request.Context(), tx, uid)
			if err != nil || !root {
				return fmt.Errorf("key rotation singleton Root authorization revoked")
			}
		}
		svc := *h.Svc
		svc.DB = tx
		resourceID, err := mutate(&svc)
		if err != nil {
			return err
		}
		return svc.OpLogs.WriteTx(c, tx, operationlog.WriteOpts{
			Action: "key_rotation_" + action, Resource: "key_rotation", ResourceID: resourceID,
			Permission: adminperm.PermSecurityKeyRotate, Status: "success", Message: "global key rotation control",
		})
	})
}

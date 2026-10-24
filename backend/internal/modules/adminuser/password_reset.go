package adminuser

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResetPassword generates a single-use display credential, replacing the old
// password and invalidating all existing access/refresh tokens by version.
// Password is not logged, persisted in plaintext, or returned by read APIs.
func (s *Service) ResetPassword(c *gin.Context, userID uuid.UUID, actorID *uuid.UUID) (string, error) {
	if s == nil || s.DB == nil || actorID == nil || *actorID == uuid.Nil || *actorID == userID {
		return "", inputError("无法重置当前账号的密码")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return "", err
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	// Keep all 192 random bits while guaranteeing the same character policy as
	// manually assigned passwords, regardless of the random base64 characters.
	temporary := "Tm2!" + base64.RawURLEncoding.EncodeToString(secret)
	hashed, err := hashPassword(temporary)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var user admin.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", userID, tenantID).First(&user).Error; err != nil {
			return err
		}
		if strings.EqualFold(user.Role, adminperm.RoleAdmin) && !strings.EqualFold(user.Status, admin.StatusActive) {
			return inputError("管理员账号已禁用")
		}
		if err := tx.Model(&user).Updates(map[string]any{
			"password_hash": hashed, "password_changed_at": now, "must_change_password": true,
			"token_version": gorm.Expr("token_version + 1"),
		}).Error; err != nil {
			return err
		}
		return writeSensitiveAudit(tx, s.OpLog, c, actorID, "user.password.reset", "admin_user", userID.String(), "temporary credential issued; prior tokens invalidated")
	})
	if err != nil {
		return "", err
	}
	adminperm.InvalidateUserPermissionCache(userID)
	return temporary, nil
}

// ResetPassword requires the same server-side user-management permission as
// editing users; return the generated secret only once in this response.
func (h *Handler) ResetPassword(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	userID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "无效的用户 ID")
		return
	}
	secret, err := h.Svc.ResetPassword(c, userID, adminUUID(c))
	if err != nil {
		handleMutationError(c, err)
		return
	}
	response.OK(c, gin.H{"temporaryPassword": secret})
}

package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type requiredPasswordChangeInput struct {
	Account         string `json:"account" binding:"required,max=128"`
	CurrentPassword string `json:"currentPassword" binding:"required,max=128"`
	NewPassword     string `json:"newPassword" binding:"required,min=12,max=72"`
}

// ChangeRequiredPassword is the only pre-auth path for accounts marked as
// requiring a new password. It issues no session or bearer credentials.
func (h *Handler) ChangeRequiredPassword(c *gin.Context) {
	if h == nil || h.Cfg == nil || h.DB == nil || h.Admins == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeInternalError, "auth unavailable")
		return
	}
	var body requiredPasswordChangeInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "密码至少12位且不超过72位")
		return
	}
	account := strings.TrimSpace(body.Account)
	if account == "" || body.CurrentPassword == body.NewPassword {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "新密码必须与临时密码不同")
		return
	}
	guard := &LoginGuard{Cfg: h.Cfg, DB: h.DB}
	if err := guard.CheckAllowed(c.Request.Context(), account, c.ClientIP()); err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, err.Error())
		return
	}
	invalid := func() {
		_ = guard.RecordFailure(c.Request.Context(), account, c.ClientIP())
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, ErrInvalidCredentials)
	}
	user, err := h.Admins.ByLoginAccount(c.Request.Context(), account)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			invalid()
			return
		}
		response.HandleError(c, err)
		return
	}
	if !strings.EqualFold(user.Status, admin.StatusActive) || !user.MustChangePassword {
		invalid()
		return
	}
	// Early verification avoids acquiring a row lock for obviously invalid passwords.
	if err := admin.CheckPassword(user.PasswordHash, body.CurrentPassword); err != nil {
		invalid()
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	now := time.Now().UTC()
	err = h.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var locked admin.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", user.ID, user.TenantID).First(&locked).Error; err != nil {
			return err
		}
		if !strings.EqualFold(locked.Status, admin.StatusActive) ||
			!locked.MustChangePassword ||
			admin.CheckPassword(locked.PasswordHash, body.CurrentPassword) != nil {
			return errors.New(ErrInvalidCredentials)
		}
		return tx.Model(&locked).Updates(map[string]any{
			"password_hash":        string(hash),
			"password_changed_at":  now,
			"must_change_password": false,
			"token_version":        gorm.Expr("token_version + 1"),
		}).Error
	})
	if err != nil && err.Error() == ErrInvalidCredentials {
		invalid()
		return
	}
	if err != nil {
		response.HandleError(c, err)
		return
	}
	_ = guard.ClearFailures(c.Request.Context(), account, c.ClientIP())
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &user.ID,
			Action:      "user.password.change_required",
			Resource:    "auth",
			ResourceID:  user.ID.String(),
			Status:      "success",
			Message:     "required password change completed",
		})
	}
	response.OK(c, gin.H{"ok": true})
}

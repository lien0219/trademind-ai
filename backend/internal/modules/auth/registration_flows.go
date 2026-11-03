package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/passwordpolicy"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AuthEmailChallenge stores a keyed digest of a short-lived email code.
// No code, reset URL, or raw invitation token is persisted or audited.
type AuthEmailChallenge struct {
	ID             uuid.UUID `gorm:"type:char(36);primaryKey"`
	Email          string    `gorm:"size:128;not null;uniqueIndex:ux_auth_challenge_email_purpose,priority:1"`
	Purpose        string    `gorm:"size:32;not null;uniqueIndex:ux_auth_challenge_email_purpose,priority:2"`
	CodeHash       string    `gorm:"size:64;not null"`
	InvitationHash string    `gorm:"size:64"`
	FailedAttempts int       `gorm:"not null;default:0"`
	SentAt         time.Time `gorm:"not null"`
	ExpiresAt      time.Time `gorm:"not null;index"`
}

func (AuthEmailChallenge) TableName() string { return "auth_email_challenges" }

type AuthEmailRate struct {
	KeyHash   string    `gorm:"size:64;primaryKey"`
	Count     int       `gorm:"not null;default:0"`
	ExpiresAt time.Time `gorm:"not null;index"`
}

func (AuthEmailRate) TableName() string { return "auth_email_rates" }

const emailCodeTTL = 15 * time.Minute

func normalizedAuthEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	parsed, err := mail.ParseAddress(email)
	return email, err == nil && parsed != nil && parsed.Address == email &&
		len(email) <= 128 && len(email) > 3 && strings.Count(email, "@") == 1 && strings.Contains(strings.SplitN(email, "@", 2)[1], ".")
}

func (h *Handler) emailChallengeKey() []byte {
	if h == nil || h.Cfg == nil {
		return nil
	}
	key := strings.TrimSpace(h.Cfg.Auth.AppMasterActiveKey)
	if key == "" {
		key = strings.TrimSpace(h.Cfg.MasterKey)
	}
	return []byte(key)
}

func hashEmailCode(key []byte, email, purpose, code string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(email + "\n" + purpose + "\n" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomEmailCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (h *Handler) PublicRegistrationPolicy(c *gin.Context) {
	response.OK(c, gin.H{"mode": settings.RegistrationPolicy(c.Request.Context(), h.DB)})
}

func invitationForEmail(tx *gorm.DB, rawToken, email string) (*admin.TenantInvitation, error) {
	if len(rawToken) < 24 || len(rawToken) > 256 {
		return nil, errors.New("invalid invitation")
	}
	hash := authutil.HashToken(rawToken, "trademind-tenant-invitation")
	var inv admin.TenantInvitation
	if err := tx.Where("token_hash = ? AND status = ? AND expires_at > ?", hash, admin.InvitationPending, time.Now().UTC()).Take(&inv).Error; err != nil {
		return nil, errors.New("invalid invitation")
	}
	if !strings.EqualFold(inv.Email, email) || inv.TenantID <= 0 {
		return nil, errors.New("invalid invitation")
	}
	var tenant admin.Tenant
	if err := tx.Where("id = ? AND status = ? AND code <> ?", inv.TenantID, admin.TenantActive, admin.RootWorkspaceCode).Take(&tenant).Error; err != nil {
		return nil, errors.New("invalid invitation")
	}
	return &inv, nil
}

func (h *Handler) rateEmailChallenge(tx *gorm.DB, key string, limit int) error {
	hash := sha256.Sum256([]byte(key))
	digest := hex.EncodeToString(hash[:])
	now := time.Now().UTC()
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&AuthEmailRate{
		KeyHash: digest, Count: 0, ExpiresAt: now.Add(time.Hour),
	}).Error; err != nil {
		return err
	}
	var rate AuthEmailRate
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rate, "key_hash = ?", digest).Error; err != nil {
		return err
	}
	if !rate.ExpiresAt.After(now) {
		rate.Count = 0
		rate.ExpiresAt = now.Add(time.Hour)
	}
	if rate.Count >= limit {
		return errors.New("too many email requests")
	}
	return tx.Model(&rate).Updates(map[string]any{"count": rate.Count + 1, "expires_at": rate.ExpiresAt}).Error
}

func (h *Handler) sendEmailChallenge(c *gin.Context) {
	var input struct {
		Email           string `json:"email"`
		Scene           string `json:"scene"`
		Locale          string `json:"locale"`
		InvitationToken string `json:"invitationToken"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid email request")
		return
	}
	email, ok := normalizedAuthEmail(input.Email)
	if !ok || (input.Scene != "register" && input.Scene != "password_reset") {
		response.Fail(c, 400, response.CodeBadRequest, "invalid email request")
		return
	}
	mode := settings.RegistrationPolicy(c.Request.Context(), h.DB)
	if input.Scene == "register" && mode == settings.RegistrationClosed {
		response.Fail(c, 403, response.CodeForbidden, "registration closed")
		return
	}
	if h == nil || h.DB == nil || h.Settings == nil || len(h.emailChallengeKey()) < 16 {
		response.Fail(c, 503, response.CodeInternalError, "email authentication not configured")
		return
	}
	code, err := randomEmailCode()
	if err != nil {
		response.HandleError(c, err)
		return
	}
	shouldSend := true
	hash := hashEmailCode(h.emailChallengeKey(), email, input.Scene, code)
	inviteHash := ""
	err = h.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := h.rateEmailChallenge(tx, "email:"+email+":"+input.Scene, 5); err != nil {
			return err
		}
		if err := h.rateEmailChallenge(tx, "ip:"+c.ClientIP(), 12); err != nil {
			return err
		}
		if input.Scene == "register" && mode == settings.RegistrationInviteOnly {
			inv, err := invitationForEmail(tx, input.InvitationToken, email)
			if err != nil {
				return err
			}
			inviteHash = inv.TokenHash
		}
		var user admin.AdminUser
		err := tx.Where("LOWER(TRIM(email)) = ?", email).Take(&user).Error
		if err == nil {
			shouldSend = input.Scene == "password_reset" && user.Status == admin.StatusActive
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else if input.Scene == "password_reset" {
			shouldSend = false
		}
		if !shouldSend {
			return nil
		}
		now := time.Now().UTC()
		var existing AuthEmailChallenge
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ? AND purpose = ?", email, input.Scene).Take(&existing).Error
		if err == nil && now.Sub(existing.SentAt) < time.Minute {
			return errors.New("verification cooldown")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row := AuthEmailChallenge{
			ID: uuid.New(), Email: email, Purpose: input.Scene, CodeHash: hash,
			InvitationHash: inviteHash, SentAt: now, ExpiresAt: now.Add(emailCodeTTL),
		}
		if err == nil {
			return tx.Model(&existing).Updates(map[string]any{
				"code_hash": hash, "invitation_hash": inviteHash,
				"failed_attempts": 0, "sent_at": now, "expires_at": row.ExpiresAt,
			}).Error
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		if err.Error() == "verification cooldown" || err.Error() == "too many email requests" {
			response.Fail(c, 429, response.CodeBadRequest, "please retry later")
		} else {
			response.Fail(c, 400, response.CodeBadRequest, "unable to send verification email")
		}
		return
	}
	if shouldSend {
		event := "auth.verify_code"
		if input.Scene == "password_reset" {
			event = "auth.password_reset"
		}
		err = h.Settings.SendEventEmail(c.Request.Context(), event, input.Locale, email, map[string]string{
			"verification_code": code, "expires_in_minutes": "15",
		})
		if err != nil {
			_ = h.DB.WithContext(c.Request.Context()).Where("email = ? AND purpose = ? AND code_hash = ?", email, input.Scene, hash).
				Delete(&AuthEmailChallenge{}).Error
			response.Fail(c, 503, response.CodeInternalError, "email delivery unavailable")
			return
		}
	}
	response.OK(c, gin.H{"ok": true})
}

// consumeEmailCode records failed attempts even when a code is wrong.
// A successful code is consumed atomically before provisioning; a failed
// provisioning operation must request a new code (fail-closed behavior).
func consumeEmailCode(ctx context.Context, db *gorm.DB, key []byte, email, purpose, code, inviteHash string) error {
	if len(code) != 6 || len(key) < 16 {
		return errors.New("verification failed")
	}
	valid := false
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var challenge AuthEmailChallenge
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("email = ? AND purpose = ?", email, purpose).Take(&challenge).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if now.After(challenge.ExpiresAt) || challenge.FailedAttempts >= 5 {
			return nil
		}
		expected := hashEmailCode(key, email, purpose, code)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(challenge.CodeHash)) != 1 ||
			(purpose == "register" && challenge.InvitationHash != inviteHash) {
			return tx.Model(&challenge).Update("failed_attempts", gorm.Expr("failed_attempts + 1")).Error
		}
		if err := tx.Delete(&challenge).Error; err != nil {
			return err
		}
		valid = true
		return nil
	})
	if err != nil || !valid {
		return errors.New("verification failed")
	}
	return nil
}

func (h *Handler) registerAccount(c *gin.Context) {
	var input struct {
		Email           string `json:"email"`
		Code            string `json:"code"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
		InvitationToken string `json:"invitationToken"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid registration request")
		return
	}
	email, ok := normalizedAuthEmail(input.Email)
	mode := settings.RegistrationPolicy(c.Request.Context(), h.DB)
	if mode == settings.RegistrationClosed {
		response.Fail(c, 403, response.CodeForbidden, "registration closed")
		return
	}
	if !ok || input.Password != input.ConfirmPassword || passwordpolicy.Validate(input.Password) != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid registration details")
		return
	}
	if h == nil || h.DB == nil || h.OpLog == nil || len(h.emailChallengeKey()) < 16 {
		response.Fail(c, 503, response.CodeInternalError, "registration unavailable")
		return
	}
	inviteHash := ""
	if mode == settings.RegistrationInviteOnly {
		inviteHash = authutil.HashToken(input.InvitationToken, "trademind-tenant-invitation")
	}
	if err := consumeEmailCode(c.Request.Context(), h.DB, h.emailChallengeKey(), email, "register", input.Code, inviteHash); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "verification failed or expired")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	err = h.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if settings.RegistrationPolicy(c.Request.Context(), tx) != mode {
			return errors.New("registration policy changed")
		}
		var inv *admin.TenantInvitation
		if mode == settings.RegistrationInviteOnly {
			var err error
			inv, err = invitationForEmail(tx, input.InvitationToken, email)
			if err != nil {
				return err
			}
		}
		keyHash := sha256.Sum256([]byte("identity:" + email))
		lock := admin.AdminMutationLock{KeyHash: hex.EncodeToString(keyHash[:])}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&lock).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lock, "key_hash = ?", lock.KeyHash).Error; err != nil {
			return err
		}
		var existing admin.AdminUser
		if err := tx.Where("LOWER(TRIM(email)) = ?", email).Take(&existing).Error; err == nil {
			return errors.New("account already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		newUser := admin.AdminUser{
			Username: admin.NewInternalUsername(), Email: email, PasswordHash: string(hash),
			DisplayName: strings.SplitN(email, "@", 2)[0],
			Role: "readonly", Status: admin.StatusActive, TokenVersion: 1,
		}
		if err := tx.Create(&newUser).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		var tenantID int64
		if mode == settings.RegistrationOpen {
			tenant := admin.Tenant{
				Code: "self-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20],
				Name: "个人工作空间", Status: admin.TenantActive,
				OwnerUserID: &newUser.ID, CreatedBy: &newUser.ID, ActivatedAt: &now, Version: 1,
				SettingsJSON: "{}",
			}
			if err := tx.Create(&tenant).Error; err != nil {
				return err
			}
			tenantID = tenant.ID
			newUser.Role = adminperm.RoleAdmin
		} else {
			tenantID = inv.TenantID
			if adminperm.IsCustomRole(inv.Role) {
				var role admin.RoleDefinition
				if err := tx.Where("tenant_id = ? AND code = ? AND enabled = ?", tenantID, inv.Role, true).Take(&role).Error; err != nil {
					return err
				}
			}
			newUser.Role = inv.Role
			var grants []admin.UserStorePermission
			if err := json.Unmarshal([]byte(inv.StoreGrantsJSON), &grants); err != nil {
				return err
			}
			for i := range grants {
				var count int64
				if err := tx.Table("shops").Where("id = ? AND tenant_id = ?", grants[i].StoreID, tenantID).Count(&count).Error; err != nil || count != 1 {
					return errors.New("invalid invitation store scope")
				}
				if grants[i].PermissionScope != admin.StorePermScopeView &&
					grants[i].PermissionScope != admin.StorePermScopeOperate &&
					grants[i].PermissionScope != admin.StorePermScopeManage {
					return errors.New("invalid store permission")
				}
				grants[i].ID = uuid.Nil
				grants[i].UserID = newUser.ID
				if err := tx.Create(&grants[i]).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(inv).Updates(map[string]any{
				"status": admin.InvitationAccepted, "accepted_by": newUser.ID, "accepted_at": now,
			}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&admin.TenantMembership{
			TenantID: tenantID, UserID: newUser.ID, Role: newUser.Role, Status: admin.MembershipActive,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&newUser).Updates(map[string]any{
			"tenant_id": tenantID, "last_tenant_id": tenantID, "role": newUser.Role,
		}).Error; err != nil {
			return err
		}
		return h.OpLog.WriteTxContext(c.Request.Context(), tx, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: &newUser.ID,
			Action: "auth.self_register", Resource: "admin_user",
			ResourceID: newUser.ID.String(), Status: "success",
		})
	})
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "registration failed or verification expired")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) ResetPasswordWithEmail(c *gin.Context) {
	var input struct {
		Email           string `json:"email"`
		Code            string `json:"code"`
		NewPassword     string `json:"newPassword"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid reset request")
		return
	}
	email, ok := normalizedAuthEmail(input.Email)
	if !ok || input.NewPassword != input.ConfirmPassword || passwordpolicy.Validate(input.NewPassword) != nil ||
		h == nil || h.DB == nil || h.OpLog == nil || len(h.emailChallengeKey()) < 16 {
		response.Fail(c, 400, response.CodeBadRequest, "invalid reset details")
		return
	}
	if err := consumeEmailCode(c.Request.Context(), h.DB, h.emailChallengeKey(), email, "password_reset", input.Code, ""); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "verification failed or expired")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	err = h.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var user admin.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("LOWER(TRIM(email)) = ? AND status = ?", email, admin.StatusActive).Take(&user).Error; err != nil {
			return errors.New("invalid reset details")
		}
		isRoot, err := adminperm.IsRoot(c.Request.Context(), tx, user.ID)
		if err != nil || isRoot {
			return errors.New("root account requires administrator recovery")
		}
		if err := tx.Model(&user).Updates(map[string]any{
			"password_hash": string(hash),
			"password_changed_at": time.Now().UTC(),
			"must_change_password": false,
			"token_version": gorm.Expr("token_version + 1"),
		}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&AuthSession{}).Where("user_id = ? AND status = ?", user.ID, SessionStatusActive).
			Updates(map[string]any{"status": SessionStatusRevoked, "revoked_at": now, "revoke_reason": "password_reset"}).Error; err != nil {
			return err
		}
		if err := tx.Model(&AuthRefreshToken{}).Where("user_id = ? AND status IN ?", user.ID, []string{RefreshStatusActive, RefreshStatusRotated}).
			Updates(map[string]any{"status": RefreshStatusRevoked, "revoked_at": now, "revoke_reason": "password_reset"}).Error; err != nil {
			return err
		}
		return h.OpLog.WriteTxContext(c.Request.Context(), tx, operationlog.WriteOpts{
			TenantID: user.TenantID, AdminUserID: &user.ID,
			Action: "auth.password_reset", Resource: "admin_user", ResourceID: user.ID.String(), Status: "success",
		})
	})
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "verification failed or expired")
		return
	}
	response.OK(c, gin.H{"ok": true})
}

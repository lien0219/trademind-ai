package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/runtimediag"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SessionService manages sessions and refresh token rotation.
type SessionService struct {
	Cfg     *config.Config
	DB      *gorm.DB
	Admins  *admin.Store
	Metrics *metrics.Catalog
	OpLog   *operationlog.Service
}

// LoginSessionResult is returned after successful credential verification.
type LoginSessionResult struct {
	AccessToken  string
	AccessExp    time.Time
	RefreshToken string
	SessionID    uuid.UUID
	User         userView
}

// CreateSession authenticates and creates session + refresh token family.
func (s *SessionService) CreateSession(ctx context.Context, account, password, ip, userAgent string) (*LoginSessionResult, error) {
	if s == nil || s.Admins == nil || s.Cfg == nil || s.DB == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	guard := &LoginGuard{Cfg: s.Cfg, DB: s.DB}
	stageStart := time.Now()
	if err := guard.CheckAllowed(ctx, account, ip); err != nil {
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "rate_limit_check", authOutcome(err), stageStart)
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "lockout_evaluate", authOutcome(err), stageStart)
		if err.Error() == ErrAccountTemporarilyLocked {
			runtimediag.Path(runtimediag.RouteAuthInvalidLogin, "locked_account")
			runtimediag.Path(runtimediag.RouteAuthInvalidLogin, runtimediag.PathLockedAccount)
			runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "lockedAccountQueryCount", 1)
		} else if err.Error() == ErrTooManyAttempts {
			runtimediag.Path(runtimediag.RouteAuthInvalidLogin, "rate_limited")
		}
		return nil, err
	}
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "rate_limit_check", runtimediag.OutcomeSuccess, stageStart)
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "lockout_evaluate", runtimediag.OutcomeSuccess, stageStart)

	stageStart = time.Now()
	var u *admin.AdminUser
	timing, err := runtimediag.TimedGorm(s.DB, func() error {
		var lookupErr error
		u, lookupErr = s.Admins.ByLoginAccount(ctx, account)
		return lookupErr
	})
	outcome := authOutcome(err)
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "account_lookup", outcome, stageStart)
	runtimediag.ObserveDBOperation(runtimediag.RouteAuthInvalidLogin, "account_lookup", outcome, stageStart)
	runtimediag.ObserveSQL(runtimediag.RouteAuthInvalidLogin, "auth", "auth.account_lookup", "select", "admin_users", outcome, false, timing)
	runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "accountLookupCount", 1)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			runtimediag.Path(runtimediag.RouteAuthInvalidLogin, "account_missing")
			runtimediag.Path(runtimediag.RouteAuthInvalidLogin, runtimediag.PathUnknownAccount)
			runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "unknownAccountQueryCount", 1)
			runtimediag.ObservePasswordVerify(runtimediag.PathUnknownAccount, runtimediag.PasswordAlgoBcrypt, bcrypt.DefaultCost, 0, time.Now())
			_ = guard.RecordFailure(ctx, account, ip)
			return nil, errors.New(ErrInvalidCredentials)
		}
		return nil, err
	}
	stageStart = time.Now()
	cost := bcrypt.DefaultCost
	if c, cerr := bcrypt.Cost([]byte(u.PasswordHash)); cerr == nil {
		cost = c
	}
	if err := admin.CheckPassword(u.PasswordHash, password); err != nil {
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "password_verify", runtimediag.OutcomeExpectedRejection, stageStart)
		runtimediag.ObservePasswordVerify(runtimediag.PathKnownWrongPassword, runtimediag.PasswordAlgoBcrypt, cost, 1, stageStart)
		runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "passwordVerifyCount", 1)
		runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "wrongPasswordQueryCount", 1)
		runtimediag.Path(runtimediag.RouteAuthInvalidLogin, "wrong_password")
		runtimediag.Path(runtimediag.RouteAuthInvalidLogin, runtimediag.PathKnownWrongPassword)
		_ = guard.RecordFailure(ctx, account, ip)
		return nil, errors.New(ErrInvalidCredentials)
	}
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "password_verify", runtimediag.OutcomeSuccess, stageStart)
	runtimediag.ObservePasswordVerify(runtimediag.PathSuccessVerify, runtimediag.PasswordAlgoBcrypt, cost, 1, stageStart)
	runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "passwordVerifyCount", 1)
	if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
		return nil, errors.New(ErrUserDisabled)
	}
	if u.MustChangePassword {
		return nil, errors.New(ErrPasswordChangeRequired)
	}
	tenantID, err := resolveLoginTenant(ctx, s.DB, u)
	if err != nil {
		return nil, fmt.Errorf("resolve login tenant: %w", err)
	}
	_ = guard.ClearFailures(ctx, account, ip)

	now := time.Now().UTC()
	session := &AuthSession{
		ID:               uuid.New(),
		TenantID:         tenantID,
		UserID:           u.ID,
		TokenVersion:     u.TokenVersion,
		Status:           SessionStatusActive,
		BrowserSummary:   authutil.SummarizeUserAgent(userAgent),
		DeviceSummary:    "web",
		IPHash:           authutil.HashIP(ip),
		UserAgentSummary: authutil.SummarizeUserAgent(userAgent),
		LastActivityAt:   now,
	}
	familyID := uuid.New()
	refreshRaw, err := authutil.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	refreshTTL := s.Cfg.RefreshTokenTTL()
	refresh := &AuthRefreshToken{
		TenantID:         tenantID,
		UserID:           u.ID,
		SessionID:        session.ID,
		TokenFamilyID:    familyID,
		TokenHash:        authutil.HashToken(refreshRaw, s.Cfg.JWTSecret),
		Status:           RefreshStatusActive,
		ExpiresAt:        now.Add(refreshTTL),
		IPHash:           authutil.HashIP(ip),
		UserAgentSummary: authutil.SummarizeUserAgent(userAgent),
	}

	ks, err := BuildKeySet(s.Cfg)
	if err != nil {
		return nil, err
	}
	access, exp, err := MintAccessToken(s.Cfg, ks, MintAccessInput{
		UserID:       u.ID,
		Username:     u.LoginLabel(),
		TenantID:     tenantID,
		SessionID:    session.ID,
		TokenVersion: u.TokenVersion,
	})
	if err != nil {
		return nil, err
	}

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		registry, memberships := adminperm.TenantSchemaState(tx)
		if tenantID > 0 && registry && memberships {
			if err := adminperm.LockTenantMutation(tx, tenantID); err != nil {
				return fmt.Errorf("lock login tenant %d: %w", tenantID, err)
			}
			access, err := adminperm.ResolveTenantAccess(ctx, tx, u.ID, tenantID)
			if err != nil {
				return fmt.Errorf("validate login tenant %d: %w", tenantID, err)
			}
			if access == nil || !access.Schema || !access.Allowed {
				return errors.New(ErrSessionRevoked)
			}
		}
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		refresh.SessionID = session.ID
		return tx.Create(refresh).Error
	})
	if err != nil {
		return nil, err
	}
	s.ObserveAuth("session_created", "success", "created", "password")

	label := u.LoginLabel()
	dn := u.DisplayName
	if dn == "" {
		dn = label
	}
	return &LoginSessionResult{
		AccessToken:  access,
		AccessExp:    exp,
		RefreshToken: refreshRaw,
		SessionID:    session.ID,
		User: userView{
			ID:          u.ID.String(),
			Username:    label,
			Email:       u.Email,
			Phone:       u.Phone,
			DisplayName: dn,
		},
	}, nil
}

// SwitchTenant revokes the calling session and refresh family, then creates a
// fresh session pair bound to an active membership in targetTenantID. Legacy
// bearer callers have no persisted session, so their global token version is
// bumped before the new server session is issued.
func (s *SessionService) SwitchTenant(ctx context.Context, userID uuid.UUID, currentSessionID uuid.UUID, currentTokenVersion int, targetTenantID int64, ip, userAgent string) (*LoginSessionResult, error) {
	if s == nil || s.Cfg == nil || s.DB == nil || userID == uuid.Nil || targetTenantID < 0 {
		return nil, fmt.Errorf("auth: invalid tenant switch")
	}
	var result *LoginSessionResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if targetTenantID > 0 {
			if err := adminperm.LockTenantMutation(tx, targetTenantID); err != nil {
				return err
			}
		}
		var user admin.AdminUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "tenant_id", "last_tenant_id", "email", "phone", "display_name", "password_hash", "role", "status", "token_version", "must_change_password").
			First(&user, "id = ?", userID).Error; err != nil {
			return errors.New(ErrAuthenticationRequired)
		}
		if !strings.EqualFold(strings.TrimSpace(user.Status), admin.StatusActive) {
			return errors.New(ErrUserDisabled)
		}
		if user.MustChangePassword {
			return errors.New(ErrPasswordChangeRequired)
		}
		root, err := adminperm.IsRoot(ctx, tx, userID)
		if err != nil {
			return err
		}
		if targetTenantID == 0 {
			if !adminperm.IsSystemAdministrator(&user) {
				return errors.New(ErrTenantAccessDenied)
			}
		} else {
			access, err := adminperm.ResolveTenantAccess(ctx, tx, userID, targetTenantID)
			if err != nil {
				return err
			}
			if access == nil || !access.Schema || !access.Allowed {
				return errors.New(ErrSessionRevoked)
			}
		}
		_ = root // root access is strictly resolved by ResolveTenantAccess

		if currentSessionID != uuid.Nil {
			var current AuthSession
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", currentSessionID).Error; err != nil || current.UserID != userID || current.Status != SessionStatusActive {
				return errors.New(ErrSessionRevoked)
			}
			if currentTokenVersion > 0 && current.TokenVersion != currentTokenVersion {
				return errors.New(ErrSessionRevoked)
			}
			if current.TokenVersion == 0 || current.TokenVersion != user.TokenVersion {
				return errors.New(ErrSessionRevoked)
			}
			var family AuthRefreshToken
			if err := tx.Where("session_id = ?", current.ID).Order("created_at DESC").First(&family).Error; err == nil {
				if err := s.revokeTokenFamilyTx(tx, family.TokenFamilyID, RefreshStatusRevoked, "tenant_switch"); err != nil {
					return err
				}
			}
			if err := s.revokeSessionTx(tx, current.ID, "tenant_switch"); err != nil {
				return err
			}
		} else {
			// Legacy bearer tokens are not persisted. Bump the identity version so
			// every old bearer token becomes unusable after this switch.
			if currentTokenVersion > 0 && user.TokenVersion != currentTokenVersion {
				return errors.New(ErrSessionRevoked)
			}
			if err := tx.Model(&user).UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
				return err
			}
			user.TokenVersion++
		}

		now := time.Now().UTC()
		session := &AuthSession{
			ID:               uuid.New(),
			TenantID:         targetTenantID,
			UserID:           user.ID,
			TokenVersion:     user.TokenVersion,
			Status:           SessionStatusActive,
			BrowserSummary:   authutil.SummarizeUserAgent(userAgent),
			DeviceSummary:    "web",
			IPHash:           authutil.HashIP(ip),
			UserAgentSummary: authutil.SummarizeUserAgent(userAgent),
			LastActivityAt:   now,
		}
		refreshRaw, err := authutil.NewOpaqueToken(32)
		if err != nil {
			return err
		}
		refresh := &AuthRefreshToken{
			TenantID:         targetTenantID,
			UserID:           user.ID,
			SessionID:        session.ID,
			TokenFamilyID:    uuid.New(),
			TokenHash:        authutil.HashToken(refreshRaw, s.Cfg.JWTSecret),
			Status:           RefreshStatusActive,
			ExpiresAt:        now.Add(s.Cfg.RefreshTokenTTL()),
			IPHash:           authutil.HashIP(ip),
			UserAgentSummary: authutil.SummarizeUserAgent(userAgent),
		}
		ks, err := BuildKeySet(s.Cfg)
		if err != nil {
			return err
		}
		accessToken, accessExp, err := MintAccessToken(s.Cfg, ks, MintAccessInput{
			UserID: user.ID, Username: user.LoginLabel(), TenantID: targetTenantID,
			SessionID: session.ID, TokenVersion: user.TokenVersion,
		})
		if err != nil {
			return err
		}
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		if err := tx.Create(refresh).Error; err != nil {
			return err
		}
		if err := tx.Model(&user).Updates(map[string]any{"last_tenant_id": targetTenantID}).Error; err != nil {
			return err
		}
		if s.OpLog == nil {
			return fmt.Errorf("auth: tenant switch audit unavailable")
		}
		if err := s.OpLog.WriteTxContext(ctx, tx, operationlog.WriteOpts{
			TenantID: targetTenantID, AdminUserID: &user.ID, SessionID: &session.ID,
			Action: "auth.tenant.switch", Resource: "auth_session", ResourceID: session.ID.String(),
			Status: "success", Message: fmt.Sprintf("targetTenantId=%d", targetTenantID),
		}); err != nil {
			return err
		}
		label := user.LoginLabel()
		dn := user.DisplayName
		if dn == "" {
			dn = label
		}
		result = &LoginSessionResult{AccessToken: accessToken, AccessExp: accessExp, RefreshToken: refreshRaw, SessionID: session.ID, User: userView{
			ID: user.ID.String(), Username: label, Email: user.Email, Phone: user.Phone, DisplayName: dn,
		}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RefreshResult holds new tokens after rotation.
type RefreshResult struct {
	AccessToken  string
	AccessExp    time.Time
	RefreshToken string
}

// RotateRefresh exchanges a refresh token for a new pair (rotation + reuse detection).
func (s *SessionService) RotateRefresh(ctx context.Context, refreshRaw, ip, userAgent string) (*RefreshResult, error) {
	if s == nil || s.Cfg == nil || s.DB == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	refreshRaw = strings.TrimSpace(refreshRaw)
	if refreshRaw == "" {
		return nil, errors.New(ErrRefreshTokenRevoked)
	}
	hash := authutil.HashToken(refreshRaw, s.Cfg.JWTSecret)
	now := time.Now().UTC()

	var result *RefreshResult
	var terminalErr error
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Discover the immutable session binding without a row lock, then use
		// the same session -> refresh lock order as switching and revocation.
		var binding AuthRefreshToken
		if err := tx.Select("session_id").Where("token_hash = ?", hash).First(&binding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(ErrRefreshTokenRevoked)
			}
			return err
		}
		var sess AuthSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&sess, "id = ?", binding.SessionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(ErrSessionRevoked)
			}
			return err
		}
		var row AuthRefreshToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ? AND session_id = ?", hash, sess.ID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(ErrRefreshTokenRevoked)
			}
			return err
		}
		if row.UserID != sess.UserID || row.TenantID != sess.TenantID {
			return errors.New(ErrSessionRevoked)
		}

		switch row.Status {
		case RefreshStatusActive:
			// proceed
		case RefreshStatusRotated:
			// Reuse detection must commit the family and session revocation before
			// returning the denial. Returning an error here would roll back the
			// security response itself.
			if err := s.revokeTokenFamilyTx(tx, row.TokenFamilyID, RefreshStatusCompromised, "reuse_detected"); err != nil {
				return err
			}
			if err := tx.Model(&AuthSession{}).Where("id = ?", row.SessionID).Updates(map[string]any{
				"status":        SessionStatusRevoked,
				"revoked_at":    &now,
				"revoke_reason": "reuse_detected",
			}).Error; err != nil {
				return err
			}
			audit := s.OpLog
			if audit == nil {
				audit = &operationlog.Service{DB: s.DB}
			}
			if err := audit.WriteTxContext(ctx, tx, operationlog.WriteOpts{
				TenantID:    row.TenantID,
				AdminUserID: &row.UserID,
				SessionID:   &row.SessionID,
				Action:      "auth.refresh.reuse_detected",
				Resource:    "auth_session",
				ResourceID:  row.SessionID.String(),
				Status:      "failure",
				Message:     fmt.Sprintf("refreshTokenId=%s tokenFamilyId=%s revokeReason=reuse_detected", row.ID, row.TokenFamilyID),
			}); err != nil {
				return err
			}
			terminalErr = errors.New(ErrRefreshTokenReused)
			return nil
		case RefreshStatusRevoked, RefreshStatusCompromised, RefreshStatusReuseDetected:
			return errors.New(ErrRefreshTokenRevoked)
		default:
			return errors.New(ErrRefreshTokenRevoked)
		}
		if now.After(row.ExpiresAt) {
			_ = tx.Model(&row).Updates(map[string]any{"status": RefreshStatusExpired}).Error
			return errors.New(ErrRefreshTokenExpired)
		}

		if sess.Status != SessionStatusActive {
			return errors.New(ErrSessionRevoked)
		}

		var u admin.AdminUser
		if err := tx.Select("id", "tenant_id", "role", "status", "token_version", "must_change_password", "email", "phone", "display_name").
			First(&u, "id = ?", row.UserID).Error; err != nil {
			return errors.New(ErrSessionRevoked)
		}
		if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
			return errors.New(ErrUserDisabled)
		}
		if u.MustChangePassword {
			return errors.New(ErrPasswordChangeRequired)
		}
		// A session minted before a role/password change cannot refresh itself
		// into the newer token version; historical unversioned sessions must re-login.
		if sess.TokenVersion == 0 || u.TokenVersion == 0 || sess.TokenVersion != u.TokenVersion {
			return errors.New(ErrSessionRevoked)
		}
		if err := validateTenantSession(ctx, tx, u.ID, sess.TenantID); err != nil {
			return err
		}

		newRaw, err := authutil.NewOpaqueToken(32)
		if err != nil {
			return err
		}
		newID := uuid.New()
		newRow := &AuthRefreshToken{
			ID:               newID,
			TenantID:         sess.TenantID,
			UserID:           row.UserID,
			SessionID:        row.SessionID,
			TokenFamilyID:    row.TokenFamilyID,
			TokenHash:        authutil.HashToken(newRaw, s.Cfg.JWTSecret),
			ParentTokenID:    &row.ID,
			Status:           RefreshStatusActive,
			ExpiresAt:        now.Add(s.Cfg.RefreshTokenTTL()),
			IPHash:           authutil.HashIP(ip),
			UserAgentSummary: authutil.SummarizeUserAgent(userAgent),
		}
		lastUsed := now
		if err := tx.Model(&row).Updates(map[string]any{
			"status":               RefreshStatusRotated,
			"replaced_by_token_id": newID,
			"last_used_at":         &lastUsed,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(newRow).Error; err != nil {
			return err
		}
		if err := tx.Model(&sess).Updates(map[string]any{
			"last_activity_at":   now,
			"ip_hash":            authutil.HashIP(ip),
			"user_agent_summary": authutil.SummarizeUserAgent(userAgent),
		}).Error; err != nil {
			return err
		}

		ks, err := BuildKeySet(s.Cfg)
		if err != nil {
			return err
		}
		access, exp, err := MintAccessToken(s.Cfg, ks, MintAccessInput{
			UserID:       u.ID,
			Username:     u.LoginLabel(),
			TenantID:     sess.TenantID,
			SessionID:    sess.ID,
			TokenVersion: u.TokenVersion,
		})
		if err != nil {
			return err
		}
		result = &RefreshResult{
			AccessToken:  access,
			AccessExp:    exp,
			RefreshToken: newRaw,
		}
		return nil
	})
	if err != nil {
		reason := classifyAuthReason(err)
		s.ObserveAuth("refresh", "failure", reason, "refresh_token")
		return nil, err
	}
	if terminalErr != nil {
		s.ObserveAuth("refresh", "failure", classifyAuthReason(terminalErr), "refresh_token")
		s.ObserveAuth("refresh_reuse", "failure", "reuse_detected", "refresh_token")
		return nil, terminalErr
	}
	s.ObserveAuth("refresh", "success", "success", "refresh_token")
	return result, nil
}

func (s *SessionService) revokeTokenFamilyTx(tx *gorm.DB, familyID uuid.UUID, status, reason string) error {
	now := time.Now().UTC()
	return tx.Model(&AuthRefreshToken{}).
		Where("token_family_id = ? AND status IN ?", familyID, []string{RefreshStatusActive, RefreshStatusRotated}).
		Updates(map[string]any{
			"status":        status,
			"revoked_at":    &now,
			"revoke_reason": reason,
		}).Error
}

func (s *SessionService) revokeSessionTx(tx *gorm.DB, sessionID uuid.UUID, reason string) error {
	now := time.Now().UTC()
	if err := tx.Model(&AuthSession{}).Where("id = ?", sessionID).Updates(map[string]any{
		"status":        SessionStatusRevoked,
		"revoked_at":    &now,
		"revoke_reason": reason,
	}).Error; err != nil {
		return err
	}
	return tx.Model(&AuthRefreshToken{}).
		Where("session_id = ? AND status = ?", sessionID, RefreshStatusActive).
		Updates(map[string]any{
			"status":        RefreshStatusRevoked,
			"revoked_at":    &now,
			"revoke_reason": reason,
		}).Error
}

// RevokeSession revokes one session and its refresh tokens.
func (s *SessionService) RevokeSession(ctx context.Context, sessionID, userID uuid.UUID, reason string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("auth: misconfigured")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sess AuthSession
		if err := tx.First(&sess, "id = ?", sessionID).Error; err != nil {
			return err
		}
		if sess.UserID != userID {
			return gorm.ErrRecordNotFound
		}
		return s.revokeSessionTx(tx, sessionID, reason)
	})
	if err == nil {
		s.ObserveAuth("session_revoked", "success", safeAuthReason(reason), "password")
	}
	return err
}

// RevokeOtherSessions revokes all sessions except current.
func (s *SessionService) RevokeOtherSessions(ctx context.Context, userID, currentSessionID uuid.UUID) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("auth: misconfigured")
	}
	var count int64
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sessions []AuthSession
		if err := tx.Where("user_id = ? AND status = ? AND id <> ?", userID, SessionStatusActive, currentSessionID).
			Find(&sessions).Error; err != nil {
			return err
		}
		for _, sess := range sessions {
			if err := s.revokeSessionTx(tx, sess.ID, "revoke_others"); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

// RevokeAllUserSessions revokes every session for a user (admin action).
func (s *SessionService) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID, reason string) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("auth: misconfigured")
	}
	var count int64
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sessions []AuthSession
		if err := tx.Where("user_id = ? AND status = ?", userID, SessionStatusActive).Find(&sessions).Error; err != nil {
			return err
		}
		for _, sess := range sessions {
			if err := s.revokeSessionTx(tx, sess.ID, reason); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

// ListSessions returns active/recent sessions for a user.
func (s *SessionService) ListSessions(ctx context.Context, userID uuid.UUID) ([]AuthSession, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	var rows []AuthSession
	err := s.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_activity_at DESC").
		Limit(50).
		Find(&rows).Error
	return rows, err
}

// ValidateSessionAccess checks session is active for access token binding.
func (s *SessionService) ValidateSessionAccess(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, tokenVersion int) error {
	return s.validateSessionAccess(ctx, sessionID, userID, tokenVersion, nil)
}

// ValidateSessionTenantAccess additionally binds the signed tenant claim to
// the tenant persisted on the server session.
func (s *SessionService) ValidateSessionTenantAccess(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, tokenVersion int, tenantID int64) error {
	return s.validateSessionAccess(ctx, sessionID, userID, tokenVersion, &tenantID)
}

func (s *SessionService) validateSessionAccess(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, tokenVersion int, tenantID *int64) error {
	if sessionID == uuid.Nil {
		return nil
	}
	if s == nil || s.DB == nil {
		return fmt.Errorf("auth: misconfigured")
	}
	var sess AuthSession
	if err := s.DB.WithContext(ctx).Select("id", "tenant_id", "status", "user_id", "token_version").First(&sess, "id = ?", sessionID).Error; err != nil {
		return errors.New(ErrSessionRevoked)
	}
	if sess.Status != SessionStatusActive || sess.UserID != userID {
		return errors.New(ErrSessionRevoked)
	}
	if tenantID != nil && sess.TenantID != *tenantID {
		return errors.New(ErrSessionRevoked)
	}
	var u admin.AdminUser
	if err := s.DB.WithContext(ctx).Select("token_version", "status", "must_change_password").First(&u, "id = ?", userID).Error; err != nil {
		return errors.New(ErrSessionRevoked)
	}
	if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
		return errors.New(ErrUserDisabled)
	}
	if u.MustChangePassword {
		return errors.New(ErrPasswordChangeRequired)
	}
	if sess.TokenVersion == 0 || u.TokenVersion == 0 || sess.TokenVersion != u.TokenVersion {
		return errors.New(ErrSessionRevoked)
	}
	if tokenVersion > 0 && tokenVersion != u.TokenVersion {
		return errors.New(ErrSessionRevoked)
	}
	if tenantID != nil {
		if err := validateTenantSession(ctx, s.DB, userID, sess.TenantID); err != nil {
			return err
		}
	}
	return nil
}

// RevokeByRefreshToken revokes session linked to refresh token (logout).
func (s *SessionService) RevokeByRefreshToken(ctx context.Context, refreshRaw string) error {
	if s == nil || s.DB == nil || s.Cfg == nil {
		return fmt.Errorf("auth: misconfigured")
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := s.revokeByRefreshTokenTx(ctx, tx, refreshRaw, nil, nil)
		return err
	})
}

// revokeByRefreshTokenTx looks up and revokes a refresh-token session inside
// the caller's transaction. A missing token is idempotent; database failures
// are returned so callers cannot report a successful logout without evidence.
func (s *SessionService) revokeByRefreshTokenTx(ctx context.Context, tx *gorm.DB, refreshRaw string, userID, sessionID *uuid.UUID) (bool, error) {
	if s == nil || s.Cfg == nil || tx == nil {
		return false, fmt.Errorf("auth: misconfigured")
	}
	raw := strings.TrimSpace(refreshRaw)
	if raw == "" {
		return false, nil
	}
	hash := authutil.HashToken(raw, s.Cfg.JWTSecret)
	var row AuthRefreshToken
	if err := tx.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if userID != nil && row.UserID != *userID {
		return false, nil
	}
	if sessionID != nil && row.SessionID != *sessionID {
		return false, nil
	}
	if err := s.revokeSessionTx(tx, row.SessionID, "logout"); err != nil {
		return false, err
	}
	return true, nil
}

// ConstantTimeCompare wraps subtle.ConstantTimeCompare for tests.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

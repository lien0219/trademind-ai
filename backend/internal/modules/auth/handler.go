package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authcookie"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/runtimediag"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

// Handler serves auth HTTP API.
type Handler struct {
	LoginSvc *LoginService
	Sessions *SessionService
	Admins   *admin.Store
	OpLog    *operationlog.Service
	Redis    *rdb.Client
	Settings *settings.Service
	DB       *gorm.DB
	Cfg      *config.Config
}

// ContextTokenVersionKey carries the verified global identity version from
// middleware into security-sensitive authentication handlers.
const ContextTokenVersionKey = "auth.token_version"

type loginBody struct {
	Account  string `json:"account" binding:"required,min=1,max=128"`
	Password string `json:"password" binding:"required,min=1,max=128"`
}

// Login POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	totalStart := time.Now()
	totalOutcome := runtimediag.OutcomeSuccess
	defer func() {
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "total", totalOutcome, totalStart)
	}()
	if h == nil || h.LoginSvc == nil {
		totalOutcome = runtimediag.OutcomeError
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	var body loginBody
	stageStart := time.Now()
	if err := c.ShouldBindJSON(&body); err != nil {
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "request_read", runtimediag.OutcomeExpectedRejection, stageStart)
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "request_decode", runtimediag.OutcomeExpectedRejection, stageStart)
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "json_decode", runtimediag.OutcomeExpectedRejection, stageStart)
		totalOutcome = runtimediag.OutcomeExpectedRejection
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "request_read", runtimediag.OutcomeSuccess, stageStart)
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "request_decode", runtimediag.OutcomeSuccess, stageStart)
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "json_decode", runtimediag.OutcomeSuccess, stageStart)
	stageStart = time.Now()
	account := strings.TrimSpace(body.Account)
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "input_normalize", runtimediag.OutcomeSuccess, stageStart)
	if account == "" {
		totalOutcome = runtimediag.OutcomeExpectedRejection
		response.Fail(c, 400, response.CodeBadRequest, "account is required")
		return
	}
	res, err := h.LoginSvc.Login(c.Request.Context(), account, body.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		totalOutcome = runtimediag.OutcomeExpectedRejection
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "invalid_decision", runtimediag.OutcomeExpectedRejection, time.Now())
		if h.OpLog != nil {
			stageStart = time.Now()
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: account,
				Action:   "login",
				Resource: "auth",
				Status:   "failed",
				Message:  err.Error(),
			})
			// security_audit / operation_log / transaction_* stages are emitted inside OpLog.Write.
			runtimediag.ObserveAuditWrite(runtimediag.RouteAuthInvalidLogin, "security_audit", runtimediag.OutcomeSuccess, stageStart)
			runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "securityAuditWriteCount", 1)
			runtimediag.Count(runtimediag.RouteAuthInvalidLogin, "operationLogWriteCount", 1)
		}
		code := response.CodeUnauthorized
		msg := err.Error()
		if msg == ErrAccountTemporarilyLocked || msg == ErrTooManyAttempts {
			code = response.CodeForbidden
		}
		stageStart = time.Now()
		response.Fail(c, 401, code, msg)
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "response_encode", runtimediag.OutcomeExpectedRejection, stageStart)
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "response_write", runtimediag.OutcomeExpectedRejection, stageStart)
		return
	}
	uid, perr := uuid.Parse(res.User.ID)
	if perr == nil && h.OpLog != nil {
		stageStart = time.Now()
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &uid,
			Username:    res.User.Username,
			Action:      "login",
			Resource:    "auth",
			Status:      "success",
		})
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "operation_log", runtimediag.OutcomeSuccess, stageStart)
	}
	out := gin.H{
		"token":     res.Token,
		"expiresAt": res.ExpiresAt,
		"user":      res.User,
		"sessionMode": func() string {
			if h.Cfg != nil {
				return h.Cfg.Auth.SessionMode
			}
			return config.AuthSessionModeLegacy
		}(),
	}
	if h.Cfg != nil && h.Cfg.UsesSecureSession() {
		SetRefreshCookieResponse(c, h.Cfg, res.RefreshToken, h.Cfg.RefreshTokenTTL())
	} else if res.RefreshToken != "" {
		out["refreshToken"] = res.RefreshToken
	}
	if h.Cfg != nil && h.Cfg.Auth.SessionMode == config.AuthSessionModeLegacy {
		out["deprecatedSessionMode"] = true
	}
	stageStart = time.Now()
	response.OK(c, out)
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "response_encode", runtimediag.OutcomeSuccess, stageStart)
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "response_write", runtimediag.OutcomeSuccess, stageStart)
}

// Profile GET /api/v1/auth/profile
func (h *Handler) Profile(c *gin.Context) {
	if h == nil || h.Admins == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	idStr, ok := c.Get(ctxkey.AdminID)
	if !ok {
		response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
		return
	}
	s, _ := idStr.(string)
	uid, err := uuid.Parse(s)
	if err != nil {
		response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
		return
	}
	u, err := h.Admins.ByID(c.Request.Context(), uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
			return
		}
		response.HandleError(c, err)
		return
	}
	dn := u.DisplayName
	if dn == "" {
		dn = u.LoginLabel()
	}
	p, principalErr := adminperm.LoadPrincipal(c, h.DB)
	if principalErr != nil {
		response.HandleError(c, principalErr)
		return
	}
	perms := []string{}
	role := "readonly"
	status := strings.TrimSpace(u.Status)
	tenantID, _ := adminperm.TenantIDFromGin(c)
	tenantName := ""
	storePerms := make([]gin.H, 0)
	if p != nil {
		role = p.Role
		perms = p.Permissions
		for _, g := range p.StoreGrants {
			storePerms = append(storePerms, gin.H{
				"storeId":         g.StoreID.String(),
				"platform":        g.Platform,
				"permissionScope": g.PermissionScope,
			})
		}
	}
	registry, memberships := adminperm.TenantSchemaState(h.DB)
	root, rootErr := adminperm.IsRoot(c.Request.Context(),h.DB,uid)
	if rootErr != nil {response.HandleError(c,rootErr);return}
	platformOperator := (p != nil && p.PlatformOperator) || root
	if p != nil && p.SystemContext {
		tenantName = "系统控制台"
	}
	if tenantID > 0 && registry && memberships {
		access, err := adminperm.ResolveTenantAccess(c.Request.Context(), h.DB, uid, tenantID)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		if access != nil {
			tenantName = access.Tenant.Name
			status = access.Membership.Status
		}
	}
	if tenantID > 0 {
		filtered := make([]string, 0, len(perms))
		for _, permission := range perms {
			if permission != adminperm.PermSecurityKeyRotate {
				filtered = append(filtered, permission)
			}
		}
		perms = filtered
	}
	response.OK(c, gin.H{
		"id":               u.ID.String(),
		"username":         u.LoginLabel(),
		"email":            u.Email,
		"phone":            u.Phone,
		"displayName":      dn,
		"role":             strings.TrimSpace(role),
		"status":           status,
		"tenantId":         tenantID,
		"tenantName":       tenantName,
		"platformOperator": platformOperator,
		"isRoot": root,
		"permissions":      perms,
		"storePermissions": storePerms,
		"createdAt":        u.CreatedAt,
		"updatedAt":        u.UpdatedAt,
	})
}

// Logout POST /api/v1/auth/logout — revokes server session when refresh cookie/body present.
func (h *Handler) Logout(c *gin.Context) {
	raw := authcookie.ReadRefresh(c)
	if raw == "" {
		var body struct {
			RefreshToken string `json:"refreshToken"`
		}
		_ = c.ShouldBindJSON(&body)
		raw = strings.TrimSpace(body.RefreshToken)
	}
	if h == nil || h.Sessions == nil || h.Sessions.DB == nil || h.Sessions.Cfg == nil || h.OpLog == nil || h.OpLog.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "logout unavailable")
		return
	}
	var uid *uuid.UUID
	if id, ok := currentUserID(c); ok {
		uid = &id
	}
	var sid *uuid.UUID
	if rawSID, ok := c.Get(ctxkey.SessionID); ok {
		if value, ok := rawSID.(string); ok {
			if parsed, err := uuid.Parse(strings.TrimSpace(value)); err == nil && parsed != uuid.Nil {
				sid = &parsed
			}
		}
	}
	err := h.Sessions.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		sessions := *h.Sessions
		sessions.DB = tx
		revoked, err := sessions.revokeByRefreshTokenTx(c.Request.Context(), tx, raw, uid, sid)
		if err != nil {
			return err
		}
		if !revoked && sid != nil && uid != nil {
			var sess AuthSession
			if err := tx.WithContext(c.Request.Context()).First(&sess, "id = ?", *sid).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			} else if sess.UserID == *uid {
				if err := sessions.revokeSessionTx(tx, *sid, "logout"); err != nil {
					return err
				}
			}
		}
		return h.OpLog.WriteTx(c, tx, operationlog.WriteOpts{
			Action:   "logout",
			Resource: "auth",
			Status:   "success",
			Message:  "logout committed",
		})
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	ClearSessionCookies(c, h.Cfg)
	response.OK(c, gin.H{"ok": true})
}

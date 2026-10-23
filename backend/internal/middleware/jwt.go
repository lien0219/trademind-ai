package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authcookie"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

// BearerAuth validates JWT and populates admin + tenant context.
func BearerAuth(cfg *config.Config) gin.HandlerFunc {
	return BearerAuthWithDB(cfg, nil, nil)
}

// BearerAuthWithDB validates JWT and optionally checks session state.
func BearerAuthWithDB(cfg *config.Config, db *gorm.DB, sessions *auth.SessionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil {
			response.Fail(c, 500, response.CodeInternalError, "auth misconfigured")
			c.Abort()
			return
		}
		h := c.GetHeader("Authorization")
		if h == "" {
			response.Fail(c, 401, response.CodeUnauthorized, auth.ErrAuthenticationRequired)
			c.Abort()
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Fail(c, 401, response.CodeUnauthorized, "invalid authorization header")
			c.Abort()
			return
		}
		raw := strings.TrimSpace(parts[1])
		claims, err := auth.ParseAccessToken(cfg, nil, raw)
		if err != nil {
			code := auth.ErrAuthenticationRequired
			if strings.Contains(strings.ToLower(err.Error()), "expired") {
				code = auth.ErrAccessTokenExpired
			}
			response.Fail(c, 401, response.CodeUnauthorized, code)
			c.Abort()
			return
		}
		uid, err := uuid.Parse(strings.TrimSpace(claims.Subject))
		if err != nil || uid == uuid.Nil {
			response.Fail(c, 401, response.CodeUnauthorized, "invalid token subject")
			c.Abort()
			return
		}
		sessID := uuid.Nil
		if s := strings.TrimSpace(claims.SessionID); s != "" {
			parsed, parseErr := uuid.Parse(s)
			if parseErr != nil {
				response.Fail(c, 401, response.CodeUnauthorized, "invalid session claim")
				c.Abort()
				return
			}
			sessID = parsed
		}
		if sessID != uuid.Nil && sessions == nil {
			response.Fail(c, 401, response.CodeUnauthorized, auth.ErrSessionRevoked)
			c.Abort()
			return
		}
		if sessID != uuid.Nil {
			if err := sessions.ValidateSessionAccess(c.Request.Context(), sessID, uid, claims.TokenVersion); err != nil {
				response.Fail(c, 401, response.CodeUnauthorized, err.Error())
				c.Abort()
				return
			}
		} else if db != nil && sessID == uuid.Nil {
			// Legacy bearer tokens have no session record. Validate the persisted
			// security version so old tokens cannot bypass a role/password reset.
			var actor admin.AdminUser
			if err := db.WithContext(c.Request.Context()).
				Select("token_version", "status", "must_change_password").
				First(&actor, "id = ?", uid).Error; err != nil ||
				!strings.EqualFold(strings.TrimSpace(actor.Status), admin.StatusActive) ||
				actor.MustChangePassword || actor.TokenVersion == 0 ||
				actor.TokenVersion != claims.TokenVersion {
				response.Fail(c, 401, response.CodeUnauthorized, auth.ErrSessionRevoked)
				c.Abort()
				return
			}
		}
		c.Set(ctxkey.AdminID, claims.Subject)
		c.Set(ctxkey.AdminUsername, claims.Username)
		tenantID := claims.TenantID
		authSource := security.AuthSourceAccessToken
		if cfg != nil {
			resolved, src, err := cfg.ResolveRequestTenantID(claims.TenantID)
			if err != nil && IsProductionLike(cfg, claims.TenantID) {
				response.Fail(c, 403, response.CodeForbidden, err.Error())
				c.Abort()
				return
			}
			if resolved > 0 {
				tenantID = resolved
			}
			if src != "" {
				authSource = src
			}
		}
		c.Set(ctxkey.TenantID, tenantID)
		if sessID != uuid.Nil {
			c.Set(ctxkey.SessionID, sessID.String())
		}
		tc := security.BuildTenantContext(c, tenantID, uid, sessID, "", nil, nil)
		tc.AuthSource = authSource
		security.SetGin(c, tc)
		c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), tc))
		c.Next()
	}
}

func IsProductionLike(cfg *config.Config, rawTenant int64) bool {
	if cfg == nil {
		return false
	}
	return config.IsStagingOrProduction(cfg.AppEnv)
}

// ReadRefreshCookie extracts refresh token from cookie.
func ReadRefreshCookie(c *gin.Context) string {
	return authcookie.ReadRefresh(c)
}

// SetRefreshCookie writes HttpOnly refresh token cookie.
func SetRefreshCookie(c *gin.Context, cfg *config.Config, token string, expires interface{}) {
	// deprecated: use authcookie.SetRefresh
}

// ClearRefreshCookie removes refresh token cookie.
func ClearRefreshCookie(c *gin.Context, cfg *config.Config) {
	authcookie.Clear(c, cfg)
}

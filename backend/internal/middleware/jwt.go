package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
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
			if err := sessions.ValidateSessionTenantAccess(c.Request.Context(), sessID, uid, claims.TokenVersion, claims.TenantID); err != nil {
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
		c.Set(auth.ContextTokenVersionKey, claims.TokenVersion)
		tenantID := claims.TenantID
		authSource := security.AuthSourceAccessToken
		independentContext := claims.TenantID == 0 && (isTenantRecoveryPath(c.Request.Method, c.Request.URL.Path) ||
			(isPlatformControlPlanePath(c.Request.URL.Path) && isPlatformOperator(c, db, uid)) ||
			(adminperm.IsSystemControlPlaneRoute(c.Request.Method, c.FullPath()) && isSystemAdministrator(c, db, uid)))
		if !independentContext {
			resolved, source, resolveErr := cfg.ResolveRequestTenantID(claims.TenantID)
			if resolveErr != nil && IsProductionLike(cfg, claims.TenantID) {
				response.Fail(c, 403, response.CodeForbidden, resolveErr.Error())
				c.Abort()
				return
			}
			if resolveErr == nil {
				tenantID = resolved
				if source != "" {
					authSource = source
				}
			}
		}
		c.Set(ctxkey.TenantID, tenantID)
		if sessID != uuid.Nil {
			c.Set(ctxkey.SessionID, sessID.String())
		}
		if !allowAuthenticatedTenantPath(c, db, uid, tenantID, cfg) {
			response.Fail(c, 403, response.CodeForbidden, "tenant access denied")
			c.Abort()
			return
		}
		tc := security.BuildTenantContext(c, tenantID, uid, sessID, "", nil, nil)
		tc.AuthSource = authSource
		security.SetGin(c, tc)
		c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), tc))
		c.Next()
	}
}

func allowAuthenticatedTenantPath(c *gin.Context, db *gorm.DB, userID uuid.UUID, tenantID int64, cfg *config.Config) bool {
	if isTenantRecoveryPath(c.Request.Method, c.Request.URL.Path) {
		return true
	}
	if isPlatformControlPlanePath(c.Request.URL.Path) {
		return isPlatformOperator(c, db, userID)
	}
	if tenantID > 0 && db != nil {
		root, rootErr := adminperm.IsRoot(c.Request.Context(), db, userID)
		if rootErr != nil {
			return false
		}
		if root {
			access, err := adminperm.ResolveTenantAccess(c.Request.Context(), db, userID, tenantID)
			return err == nil && access != nil && access.Allowed
		}
	}
	if tenantID == 0 && adminperm.IsSystemControlPlaneRoute(c.Request.Method, c.FullPath()) {
		return isSystemAdministrator(c, db, userID)
	}
	registry, memberships := adminperm.TenantSchemaState(db)
	if !registry && !memberships {
		if cfg == nil {
			return false
		}
		env := config.NormalizeEnv(cfg.AppEnv)
		if env != config.EnvDevelopment && env != config.EnvTest {
			return false
		}
		return tenantID >= 0
	}
	if tenantID <= 0 || !registry || !memberships {
		return false
	}
	access, err := adminperm.ResolveTenantAccess(c.Request.Context(), db, userID, tenantID)
	return err == nil && access != nil && access.Allowed
}

func isTenantRecoveryPath(method, path string) bool {
	switch {
	case method == "GET" && path == "/api/v1/auth/tenants",
		method == "POST" && path == "/api/v1/auth/tenants/switch",
		method == "GET" && path == "/api/v1/auth/profile",
		method == "POST" && path == "/api/v1/auth/logout",
		method == "GET" && path == "/api/v1/auth/sessions",
		method == "POST" && path == "/api/v1/auth/sessions/revoke-others",
		method == "POST" && path == "/api/v1/auth/logout-all",
		method == "POST" && path == "/api/v1/tenant/invitations/accept":
		return true
	case method == "DELETE" && strings.HasPrefix(path, "/api/v1/auth/sessions/"):
		rest := strings.TrimPrefix(path, "/api/v1/auth/sessions/")
		return rest != "" && !strings.Contains(rest, "/") && rest != "revoke-others"
	default:
		return false
	}
}

func isPlatformControlPlanePath(path string) bool {
	return path == "/api/v1/platform/tenants" || strings.HasPrefix(path, "/api/v1/platform/tenants/")
}

func isPlatformOperator(c *gin.Context, db *gorm.DB, userID uuid.UUID) bool {
	if db == nil || !db.Migrator().HasTable(&admin.PlatformOperator{}) {
		return false
	}
	var row admin.PlatformOperator
	return db.WithContext(c.Request.Context()).Where("user_id = ? AND enabled = ?", userID, true).First(&row).Error == nil
}

func isSystemAdministrator(c *gin.Context, db *gorm.DB, userID uuid.UUID) bool {
	if db == nil {
		return false
	}
	var row admin.AdminUser
	return db.WithContext(c.Request.Context()).Select("id", "tenant_id", "role", "status").First(&row, "id = ?", userID).Error == nil &&
		adminperm.IsSystemAdministrator(&row)
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

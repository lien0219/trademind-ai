package adminuser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"golang.org/x/crypto/bcrypt"
)

func TestDeleteUserInvalidatesEveryCredentialOnlyAfterCommit(t *testing.T) {
	for _, auditFails := range []bool{false, true} {
		name := "committed"
		if auditFails {
			name = "audit_rollback"
		}
		t.Run(name, func(t *testing.T) {
			svc, actor, target := securityFixture(t)
			require.NoError(t, svc.DB.AutoMigrate(&auth.AuthSession{}, &auth.AuthRefreshToken{}, &auth.AuthLoginAttempt{}))
			const password = "synthetic-password-123"
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
			require.NoError(t, err)
			require.NoError(t, svc.DB.Model(&target).Update("password_hash", string(hash)).Error)
			cfg := &config.Config{JWTSecret: "delete-regression-secret-at-least-thirty-two-bytes", Auth: config.AuthConfig{
				SessionMode: config.AuthSessionModeSecure, AccessTokenTTLMinutes: 15, RefreshTokenTTLDays: 7,
			}}
			store := &admin.Store{DB: svc.DB}
			sessions := &auth.SessionService{Cfg: cfg, DB: svc.DB, Admins: store, OpLog: svc.OpLog}
			login := &auth.LoginService{Cfg: cfg, Admins: store}
			ctx := context.Background()
			legacy, err := login.Login(ctx, target.Email, password, "127.0.0.1", "delete-regression")
			require.NoError(t, err)
			current, err := sessions.CreateSession(ctx, target.Email, password, "127.0.0.1", "current-browser")
			require.NoError(t, err)
			history, err := sessions.CreateSession(ctx, target.Email, password, "127.0.0.1", "historical-browser")
			require.NoError(t, err)
			rotated, err := sessions.RotateRefresh(ctx, current.RefreshToken, "127.0.0.1", "current-browser")
			require.NoError(t, err, "refresh must be usable before deleting the account")
			accessStatus := func(token string) int {
				router := gin.New()
				router.GET("/protected", middleware.BearerAuthWithDB(cfg, svc.DB, sessions), func(c *gin.Context) { c.Status(http.StatusOK) })
				req := httptest.NewRequest(http.MethodGet, "/protected", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				return w.Code
			}
			tokens := []string{legacy.Token, current.AccessToken, history.AccessToken}
			for _, token := range tokens {
				require.Equal(t, http.StatusOK, accessStatus(token))
			}
			if auditFails {
				failAuditInsert(t, svc.DB)
			}
			w := roleRequest(t, svc, actor, http.MethodDelete, "/api/v1/admin/users/"+target.ID.String(), nil)
			want := http.StatusUnauthorized
			if auditFails {
				require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
				want = http.StatusOK
			} else {
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			}
			for _, token := range tokens {
				require.Equal(t, want, accessStatus(token), "legacy and every session access token must follow the committed identity")
			}
			for _, session := range []*auth.LoginSessionResult{current, history} {
				err := sessions.ValidateSessionAccess(ctx, session.SessionID, target.ID, target.TokenVersion)
				if auditFails {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			}
			for _, refresh := range []string{rotated.RefreshToken, history.RefreshToken} {
				_, err := sessions.RotateRefresh(ctx, refresh, "127.0.0.1", "delete-regression")
				if auditFails {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			}
			_, err = login.Login(ctx, target.Email, password, "127.0.0.1", "delete-regression")
			if auditFails {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			replacement, err := svc.Create(securityContext(actor), CreateBody{Email: target.Email, Password: password, Role: adminperm.RoleOperator}, &actor.ID)
			require.NoError(t, err)
			require.NotEqual(t, target.ID.String(), replacement.ID)
			for _, token := range tokens {
				require.Equal(t, http.StatusUnauthorized, accessStatus(token), "reusing an account name must not revive its deleted identity")
			}
		})
	}
}

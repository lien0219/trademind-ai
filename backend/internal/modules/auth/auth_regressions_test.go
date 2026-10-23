package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestProfileLimitsKeyRotationToGlobalAdministrator(t *testing.T) {
	for _, tenant := range []int64{0, 17} {
		t.Run(fmt.Sprint(tenant), func(t *testing.T) {
			db, _, actor, _ := newAuthRegressionFixture(t, config.AuthSessionModeSecure)
			require.NoError(t, db.Model(actor).Update("tenant_id", tenant).Error)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
			c.Set(ctxkey.AdminID, actor.ID.String())
			c.Set(ctxkey.TenantID, tenant)
			(&Handler{DB: db, Admins: &admin.Store{DB: db}}).Profile(c)
			require.Equal(t, http.StatusOK, w.Code)
			var out struct {
				Data struct {
					Permissions []string `json:"permissions"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
			if tenant == 0 {
				require.Contains(t, out.Data.Permissions, adminperm.PermSecurityKeyRotate)
			} else {
				require.NotContains(t, out.Data.Permissions, adminperm.PermSecurityKeyRotate)
			}
		})
	}
}

func TestPublicRegistrationCannotProvisionAdministrators(t *testing.T) {
	for _, endpoint := range []string{"register", "send-email-code"} {
		t.Run(endpoint, func(t *testing.T) {
			db, cfg, _, _ := newAuthRegressionFixture(t, config.AuthSessionModeSecure)
			h := &Handler{Cfg: cfg, Admins: &admin.Store{DB: db}}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/"+endpoint, bytes.NewBufferString(`{"email":"new@example.test","code":"123456","password":"synthetic-password","confirmPassword":"synthetic-password","scene":"register"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			if endpoint == "register" {
				h.Register(c)
			} else {
				h.SendEmailCode(c)
			}
			require.Equal(t, http.StatusForbidden, w.Code)
			var count int64
			require.NoError(t, db.Model(&admin.AdminUser{}).Where("email = ?", "new@example.test").Count(&count).Error)
			require.Zero(t, count, "public callers must not create console identities")
		})
	}
}

func TestSessionRevocationsRequireAtomicAudit(t *testing.T) {
	for _, operation := range []string{"single", "others", "all"} {
		for _, auditState := range []string{"available", "missing", "insert failure"} {
			t.Run(operation+"/"+auditState, func(t *testing.T) {
				db, cfg, actor, sessions := newAuthRegressionFixture(t, config.AuthSessionModeSecure)
				current := AuthSession{ID: uuid.New(), UserID: actor.ID, TenantID: actor.TenantID, TokenVersion: actor.TokenVersion, Status: SessionStatusActive}
				target := AuthSession{ID: uuid.New(), UserID: actor.ID, TenantID: actor.TenantID, TokenVersion: actor.TokenVersion, Status: SessionStatusActive}
				require.NoError(t, db.Create(&current).Error)
				require.NoError(t, db.Create(&target).Error)
				refresh := AuthRefreshToken{ID: uuid.New(), UserID: actor.ID, TenantID: actor.TenantID, SessionID: target.ID, TokenFamilyID: uuid.New(), TokenHash: "synthetic-hash", Status: RefreshStatusActive, ExpiresAt: time.Now().Add(time.Hour)}
				require.NoError(t, db.Create(&refresh).Error)
				logs := &operationlog.Service{DB: db}
				if auditState == "missing" {
					logs = nil
				} else if auditState == "insert failure" {
					require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:reject_session_audit", func(tx *gorm.DB) {
						if tx.Statement.Table == "operation_logs" {
							tx.AddError(errors.New("audit storage unavailable"))
						}
					}))
				}
				h := &SessionHandler{Cfg: cfg, DB: db, Sessions: sessions, OpLog: logs}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/sessions", nil)
				c.Set(ctxkey.AdminID, actor.ID.String())
				c.Set(ctxkey.TenantID, actor.TenantID)
				c.Set(ctxkey.SessionID, current.ID.String())
				switch operation {
				case "single":
					c.Params = gin.Params{{Key: "id", Value: target.ID.String()}}
					h.DeleteSession(c)
				case "others":
					h.RevokeOthers(c)
				case "all":
					h.LogoutAll(c)
				}
				var savedSession AuthSession
				var savedRefresh AuthRefreshToken
				require.NoError(t, db.First(&savedSession, "id = ?", target.ID).Error)
				require.NoError(t, db.First(&savedRefresh, "id = ?", refresh.ID).Error)
				var auditCount int64
				require.NoError(t, db.Model(&operationlog.OperationLog{}).Count(&auditCount).Error)
				if auditState == "available" {
					require.Equal(t, http.StatusOK, w.Code)
					require.Equal(t, SessionStatusRevoked, savedSession.Status)
					require.Equal(t, RefreshStatusRevoked, savedRefresh.Status)
					require.EqualValues(t, 1, auditCount)
				} else {
					require.Equal(t, http.StatusInternalServerError, w.Code)
					require.Equal(t, SessionStatusActive, savedSession.Status)
					require.Equal(t, RefreshStatusActive, savedRefresh.Status)
					require.Zero(t, auditCount)
				}
			})
		}
	}
}

func newAuthRegressionFixture(t *testing.T, mode string) (*gorm.DB, *config.Config, *admin.AdminUser, *SessionService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:auth_regression_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}, &operationlog.OperationLog{}); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("valid-password-123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	user := &admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     42,
		Username:     "auth-regression-" + uuid.NewString(),
		Email:        "auth-regression-" + uuid.NewString() + "@example.test",
		PasswordHash: string(hash),
		Role:         admin.RoleAdmin,
		Status:       admin.StatusActive,
		TokenVersion: 7,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "auth-regression-secret-at-least-thirty-two-bytes",
		Auth: config.AuthConfig{
			SessionMode:           mode,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	sessions := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	return db, cfg, user, sessions
}

func TestRequiredPasswordChangeRollsBackWhenAuditFails(t *testing.T) {
	db, cfg, user, _ := newAuthRegressionFixture(t, config.AuthSessionModeSecure)
	oldHash := user.PasswordHash
	if err := db.Model(user).Updates(map[string]any{"must_change_password": true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Create().Before("gorm:create").Register("test:fail_required_password_audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("test audit persistence failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(map[string]string{
		"account":         user.Email,
		"currentPassword": "valid-password-123",
		"newPassword":     "new-valid-password-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-required-password", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.TraceID, "required-password-audit-regression")
	h := &Handler{
		Cfg:    cfg,
		DB:     db,
		Admins: &admin.Store{DB: db},
		OpLog:  &operationlog.Service{DB: db},
	}
	h.ChangeRequiredPassword(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	var current admin.AdminUser
	if err := db.First(&current, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.PasswordHash != oldHash || !current.MustChangePassword || current.TokenVersion != user.TokenVersion {
		t.Fatalf("required password mutation committed despite audit failure: %+v", current)
	}
}

func TestCreateSessionAccessTokenBindsPersistedSession(t *testing.T) {
	_, cfg, user, sessions := newAuthRegressionFixture(t, config.AuthSessionModeSecure)
	result, err := sessions.CreateSession(context.Background(), user.Email, "valid-password-123", "127.0.0.1", "regression")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAccessToken(cfg, nil, result.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.SessionID == "" || claims.SessionID == uuid.Nil.String() {
		t.Fatalf("access token must bind to a persisted session, got session id %q", claims.SessionID)
	}
	if claims.SessionID != result.SessionID.String() {
		t.Fatalf("access token session id = %q, result session id = %q", claims.SessionID, result.SessionID)
	}
	if err := sessions.ValidateSessionAccess(context.Background(), result.SessionID, user.ID, claims.TokenVersion); err != nil {
		t.Fatalf("new session access token rejected: %v", err)
	}
	if err := sessions.RevokeSession(context.Background(), result.SessionID, user.ID, "regression"); err != nil {
		t.Fatal(err)
	}
	if err := sessions.ValidateSessionAccess(context.Background(), result.SessionID, user.ID, claims.TokenVersion); err == nil {
		t.Fatal("revoked session access token accepted")
	}
}

func TestRefreshTokenReusePersistsFamilyAndSessionRevocation(t *testing.T) {
	db, _, user, sessions := newAuthRegressionFixture(t, config.AuthSessionModeSecure)
	first, err := sessions.CreateSession(context.Background(), user.Email, "valid-password-123", "127.0.0.1", "regression")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := sessions.RotateRefresh(context.Background(), first.RefreshToken, "127.0.0.1", "regression")
	if err != nil {
		t.Fatal(err)
	}
	for name, accessToken := range map[string]string{"original": first.AccessToken, "rotated": rotated.AccessToken} {
		claims, err := ParseAccessToken(sessions.Cfg, nil, accessToken)
		if err != nil {
			t.Fatalf("parse %s access token: %v", name, err)
		}
		if err := sessions.ValidateSessionAccess(context.Background(), first.SessionID, user.ID, claims.TokenVersion); err != nil {
			t.Fatalf("%s access token should be valid before replay: %v", name, err)
		}
	}
	if _, err := sessions.RotateRefresh(context.Background(), first.RefreshToken, "127.0.0.1", "replay"); err == nil || err.Error() != ErrRefreshTokenReused {
		t.Fatalf("replayed refresh token error = %v, want %s", err, ErrRefreshTokenReused)
	}

	var session AuthSession
	if err := db.First(&session, first.SessionID).Error; err != nil {
		t.Fatal(err)
	}
	if session.Status != SessionStatusRevoked {
		t.Fatalf("session status = %q, want %q", session.Status, SessionStatusRevoked)
	}
	var family []AuthRefreshToken
	if err := db.Where("session_id = ?", first.SessionID).Find(&family).Error; err != nil {
		t.Fatal(err)
	}
	if len(family) != 2 {
		t.Fatalf("refresh family row count = %d, want 2", len(family))
	}
	for _, row := range family {
		if row.Status != RefreshStatusCompromised {
			t.Fatalf("refresh token %s status = %q, want %q", row.ID, row.Status, RefreshStatusCompromised)
		}
	}
	if _, err := sessions.RotateRefresh(context.Background(), rotated.RefreshToken, "127.0.0.1", "after-replay"); err == nil || err.Error() != ErrRefreshTokenRevoked {
		t.Fatalf("newest refresh token after replay error = %v, want %s", err, ErrRefreshTokenRevoked)
	}
	if _, err := sessions.RotateRefresh(context.Background(), first.RefreshToken, "127.0.0.1", "after-replay"); err == nil || err.Error() != ErrRefreshTokenRevoked {
		t.Fatalf("replayed refresh token after compromise error = %v, want %s", err, ErrRefreshTokenRevoked)
	}
	for name, accessToken := range map[string]string{"original": first.AccessToken, "rotated": rotated.AccessToken} {
		claims, err := ParseAccessToken(sessions.Cfg, nil, accessToken)
		if err != nil {
			t.Fatalf("parse %s access token after replay: %v", name, err)
		}
		if err := sessions.ValidateSessionAccess(context.Background(), first.SessionID, user.ID, claims.TokenVersion); err == nil {
			t.Fatalf("%s access token remained valid after refresh replay", name)
		} else if err.Error() != ErrSessionRevoked {
			t.Fatalf("%s access token validation error = %v, want %s", name, err, ErrSessionRevoked)
		}
	}
}

func TestLegacyLoginMintsCurrentTenantAndTokenVersion(t *testing.T) {
	_, cfg, user, sessions := newAuthRegressionFixture(t, config.AuthSessionModeLegacy)
	login := &LoginService{Cfg: cfg, Admins: sessions.Admins}
	result, err := login.Login(context.Background(), user.Email, "valid-password-123", "127.0.0.1", "legacy-regression")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAccessToken(cfg, nil, result.Token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TenantID != user.TenantID {
		t.Fatalf("legacy token tenant = %d, want %d", claims.TenantID, user.TenantID)
	}
	if claims.TokenVersion != user.TokenVersion {
		t.Fatalf("legacy token version = %d, want %d", claims.TokenVersion, user.TokenVersion)
	}
}

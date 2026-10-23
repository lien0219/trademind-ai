package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestBearerAuthRejectsMalformedSessionIDButAcceptsLegacyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:jwt_malformed_session_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	user := admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     0,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "unused-test-hash",
		Role:         admin.RoleAdmin,
		Status:       admin.StatusActive,
		TokenVersion: 9,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{JWTSecret: "middleware-regression-secret-with-at-least-32-bytes"}
	for _, tc := range []struct {
		name      string
		sessionID string
		wantCode  int
	}{
		{name: "legacy token without session id", sessionID: "", wantCode: http.StatusNoContent},
		{name: "legacy token with nil session id", sessionID: uuid.Nil.String(), wantCode: http.StatusNoContent},
		{name: "malformed session id", sessionID: "not-a-uuid", wantCode: http.StatusUnauthorized},
		{name: "session-bound token without session validator", sessionID: uuid.NewString(), wantCode: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := auth.AccessClaims{
				Username:     "legacy-admin",
				TokenType:    "access",
				TenantID:     user.TenantID,
				SessionID:    tc.sessionID,
				TokenVersion: user.TokenVersion,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   user.ID.String(),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
				},
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.Use(BearerAuthWithDB(cfg, db, nil))
			router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantCode, recorder.Body.String())
			}
		})
	}
}

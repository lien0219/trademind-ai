package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"golang.org/x/crypto/bcrypt"
)

func TestRequiredPasswordChangeEnforcesPolicyWithoutChangingRejectedCredentials(t *testing.T) {
	for _, password := range []string{"中文密码Abcdef123456", "123456789012", "passwordletters", "invalid pass123", "valid-new-pass123!"} {
		t.Run(password, func(t *testing.T) {
			db, cfg, user, _ := newAuthRegressionFixture(t, config.AuthSessionModeSecure)
			// Old credentials remain verifiable even if they predate the new policy.
			const oldPassword = "旧密码123"
			hash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
			require.NoError(t, err)
			require.NoError(t, db.Model(user).Updates(map[string]any{"must_change_password": true, "password_hash": string(hash)}).Error)
			body, err := json.Marshal(map[string]string{"account": user.Email, "currentPassword": oldPassword, "newPassword": password})
			require.NoError(t, err)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-required-password", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			(&Handler{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}, OpLog: &operationlog.Service{DB: db}}).ChangeRequiredPassword(c)
			var saved admin.AdminUser
			require.NoError(t, db.First(&saved, "id = ?", user.ID).Error)
			if password == "valid-new-pass123!" {
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.False(t, saved.MustChangePassword)
				require.NoError(t, admin.CheckPassword(saved.PasswordHash, password))
				require.Equal(t, user.TokenVersion+1, saved.TokenVersion)
			} else {
				require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
				require.True(t, saved.MustChangePassword)
				require.Equal(t, string(hash), saved.PasswordHash)
				require.Equal(t, user.TokenVersion, saved.TokenVersion)
				var count int64
				require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("status = ?", "success").Count(&count).Error)
				require.Zero(t, count)
			}
		})
	}
}

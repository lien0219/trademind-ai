package adminuser

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/passwordpolicy"
)

func TestResetPasswordIssuesPolicyCompliantOneTimeCredential(t *testing.T) {
	svc, actor, target := securityFixture(t)
	password, err := svc.ResetPassword(securityContext(actor), target.ID, &actor.ID)
	require.NoError(t, err)
	require.NoError(t, passwordpolicy.Validate(password))
	var saved admin.AdminUser
	require.NoError(t, svc.DB.First(&saved, "id = ?", target.ID).Error)
	require.True(t, saved.MustChangePassword)
	require.Equal(t, target.TokenVersion+1, saved.TokenVersion)
	require.NoError(t, admin.CheckPassword(saved.PasswordHash, password))
	var audit operationlog.OperationLog
	require.NoError(t, svc.DB.Where("action = ?", "user.password.reset").First(&audit).Error)
	require.NotContains(t, audit.Message, password)
	require.NotContains(t, audit.Message, saved.PasswordHash)
}

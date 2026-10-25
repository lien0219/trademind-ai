package tenantmgmt

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestLoginCannotCreateTenantSessionAcrossSuspension(t *testing.T) {
	svc, actor := provisionFixture(t)
	require.NoError(t, svc.DB.AutoMigrate(&auth.AuthLoginAttempt{}))

	tenant := admin.Tenant{Code: "login-race", Name: "Login race", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	password := "valid-password-123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	user := admin.AdminUser{
		Base: model.Base{ID: uuid.New()}, TenantID: tenant.ID, LastTenantID: tenant.ID,
		Username: "login-race-" + uuid.NewString(), Email: fmt.Sprintf("login-race-%s@example.test", uuid.NewString()),
		PasswordHash: string(hash), Role: "admin", Status: admin.StatusActive, TokenVersion: 1,
	}
	require.NoError(t, svc.DB.Create(&user).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{TenantID: tenant.ID, UserID: user.ID, Role: "admin", Status: admin.MembershipActive}).Error)

	cfg := &config.Config{
		JWTSecret: "tenant-login-race-test-secret-at-least-thirty-two-bytes",
		Auth:      config.AuthConfig{SessionMode: config.AuthSessionModeSecure, AccessTokenTTLMinutes: 15, RefreshTokenTTLDays: 7},
	}
	sessions := &auth.SessionService{Cfg: cfg, DB: svc.DB, Admins: &admin.Store{DB: svc.DB}}
	resolved := make(chan struct{})
	resume := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(resume) }) }
	var pauseOnce sync.Once
	observedQueries := make(chan string, 32)
	queryCallbacks := svc.DB.Callback().Query()
	require.NoError(t, queryCallbacks.After("gorm:query").Register("test_pause_after_tenant_resolution", func(tx *gorm.DB) {
		table := tx.Statement.Table
		if tx.Statement.Schema != nil {
			table += " " + tx.Statement.Schema.Table
		}
		query := strings.ToLower(tx.Statement.SQL.String())
		select {
		case observedQueries <- fmt.Sprintf("table=%q membership=%t tenant_join=%t", table, strings.Contains(query, "tenant_memberships"), strings.Contains(query, "join tenants")):
		default:
		}
		if strings.Contains(query, "tenant_memberships") && strings.Contains(query, "join tenants") {
			pauseOnce.Do(func() {
				close(resolved)
				<-resume
			})
		}
	}))

	type loginResult struct {
		result *auth.LoginSessionResult
		err    error
	}
	loginDone := make(chan loginResult, 1)
	go func() {
		result, err := sessions.CreateSession(context.Background(), user.Email, password, "127.0.0.1", "tenant-login-race")
		loginDone <- loginResult{result: result, err: err}
	}()
	loginCollected := false
	defer func() {
		release()
		if !loginCollected {
			select {
			case <-loginDone:
			case <-time.After(5 * time.Second):
				t.Error("login goroutine did not finish after releasing the tenant-resolution barrier")
			}
		}
	}()

	select {
	case <-resolved:
	case <-time.After(5 * time.Second):
		t.Fatalf("login did not resolve its tenant; observed queries: %v", observedQueries)
	}
	_, err = svc.SetStatus(context.Background(), actor.ID, tenant.ID, admin.TenantSuspended, tenant.Version)
	require.NoError(t, err)
	release()
	login := <-loginDone
	loginCollected = true

	var suspended admin.Tenant
	require.NoError(t, svc.DB.First(&suspended, "id = ?", tenant.ID).Error)
	_, err = svc.SetStatus(context.Background(), actor.ID, tenant.ID, admin.TenantActive, suspended.Version)
	require.NoError(t, err)
	if login.err == nil {
		require.NotNil(t, login.result)
		require.Error(t, sessions.ValidateSessionAccess(context.Background(), login.result.SessionID, user.ID, 1), "a session created during suspension must not become usable after reactivation")
	}
	require.EqualError(t, login.err, auth.ErrSessionRevoked, "login must revalidate tenant access before persisting a session")
	var sessionCount, refreshCount int64
	require.NoError(t, svc.DB.Model(&auth.AuthSession{}).Where("user_id = ?", user.ID).Count(&sessionCount).Error)
	require.NoError(t, svc.DB.Model(&auth.AuthRefreshToken{}).Where("user_id = ?", user.ID).Count(&refreshCount).Error)
	require.Zero(t, sessionCount)
	require.Zero(t, refreshCount)
}

func TestTenantMutationsNeverAuthorizeTenantAdministrators(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_admin_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &admin.PlatformOperator{}))
	actor := uuid.New()
	svc := &Service{DB: db}
	_, err = svc.Create(context.Background(), actor, CreateTenantInput{
		Code: "company-a", Name: "Company A",
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&admin.Tenant{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSetStatusReturnsCommittedTenantState(t *testing.T) {
	svc, actor := provisionFixture(t)
	tenant := admin.Tenant{Code: "status-result", Name: "Status result", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)

	updated, err := svc.SetStatus(context.Background(), actor.ID, tenant.ID, admin.TenantSuspended, tenant.Version)
	require.NoError(t, err)
	require.Equal(t, tenant.ID, updated.ID)
	require.Equal(t, admin.TenantSuspended, updated.Status)
	require.EqualValues(t, tenant.Version+1, updated.Version)
	require.NotNil(t, updated.SuspendedAt)
}

func TestTenantMetadataRejectsInvalidInputsEvenForOperators(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_input_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &admin.PlatformOperator{}))
	actor := uuid.New()
	require.NoError(t, db.Create(&admin.PlatformOperator{UserID: actor, Enabled: true}).Error)
	svc := &Service{DB: db}
	for _, code := range []string{"", "../tenant", "admin:all", "a", "TWO WORDS"} {
		_, err := svc.Create(context.Background(), actor, CreateTenantInput{Code: code, Name: "Example"})
		require.Error(t, err, code)
	}
	var count int64
	require.NoError(t, db.Model(&admin.Tenant{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestTenantStatusFailsClosedWhenAuditUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant_status_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.Tenant{}, &admin.PlatformOperator{}))
	actor := uuid.New()
	require.NoError(t, db.Create(&admin.PlatformOperator{UserID: actor, Enabled: true}).Error)
	row := admin.Tenant{Code: "tenant-a", Name: "Tenant A", Status: admin.TenantActive}
	require.NoError(t, db.Create(&row).Error)
	svc := &Service{DB: db}
	_, err = svc.SetStatus(context.Background(), actor, row.ID, admin.TenantSuspended, row.Version)
	require.Error(t, err)
	var reloaded admin.Tenant
	require.NoError(t, db.First(&reloaded, "id = ?", row.ID).Error)
	require.Equal(t, admin.TenantActive, reloaded.Status)
}

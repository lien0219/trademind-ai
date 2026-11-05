package collectbrowserprofile

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

type profileAuditCollector struct {
	loginCalls int
	checkCalls int
}

func (c *profileAuditCollector) OpenProfileLogin(context.Context, string, string) (string, error) {
	c.loginCalls++
	return "opened", nil
}

func (c *profileAuditCollector) CheckProfileAccess(context.Context, string, string) (*CheckResultDTO, error) {
	c.checkCalls++
	return &CheckResultDTO{AccessStatus: "public"}, nil
}

func TestBrowserProfilesAreTenantScopedAndRequireWritePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &CollectBrowserProfile{}, &operationlog.OperationLog{}))
	user := admin.AdminUser{TenantID: 31, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	other := admin.AdminUser{TenantID: 32, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, db.Create(&CollectBrowserProfile{TenantID: 31, Name: "A", Domain: "a.example", ProfileKey: "profile_a", Status: StatusActive}).Error)
	require.NoError(t, db.Create(&CollectBrowserProfile{TenantID: 32, Name: "B", Domain: "b.example", ProfileKey: "profile_b", Status: StatusActive}).Error)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		current := user
		if c.GetHeader("X-Test-Tenant") == "32" {
			current = other
		}
		c.Set(ctxkey.AdminID, current.ID.String())
		c.Set(ctxkey.TenantID, current.TenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db, OpLog: &operationlog.Service{DB: db}}})

	list := httptest.NewRecorder()
	r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/collect/browser-profiles", nil))
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), `"name":"A"`)
	require.NotContains(t, list.Body.String(), `"name":"B"`)

	create := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collect/browser-profiles", bytes.NewBufferString(`{"name":"Shared","domain":"example.test"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(create, req)
	require.Equal(t, http.StatusOK, create.Code, create.Body.String())
	var created CollectBrowserProfile
	require.NoError(t, db.Where("name = ?", "Shared").First(&created).Error)
	require.EqualValues(t, 31, created.TenantID)

	otherList := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/collect/browser-profiles", nil)
	req.Header.Set("X-Test-Tenant", "32")
	r.ServeHTTP(otherList, req)
	require.Equal(t, http.StatusOK, otherList.Code, otherList.Body.String())
	require.Contains(t, otherList.Body.String(), `"name":"B"`)
	require.NotContains(t, otherList.Body.String(), `"name":"A"`)
	var count int64
	require.NoError(t, db.Model(&CollectBrowserProfile{}).Count(&count).Error)
	require.EqualValues(t, 3, count)
}

func TestBrowserProfileEnricherRejectsCrossTenantProfile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CollectBrowserProfile{}))
	id := uuid.New()
	row := CollectBrowserProfile{
		Base: model.Base{ID: id}, Name: "root only", Domain: "example.test",
		ProfileKey: "profile_" + uuid.NewString(), Status: StatusActive,
	}
	require.NoError(t, db.Create(&row).Error)
	opts := map[string]any{}
	err = (&Service{DB: db}).EnrichCollectorOptions(t.Context(), 31, opts, &id, true, "https://example.test/item")
	require.ErrorIs(t, err, ErrProfileNotFound)
	require.Empty(t, opts)
}

func TestCreateRollsBackWhenAuditCannotBePersisted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &CollectBrowserProfile{}, &operationlog.OperationLog{}))
	actor := admin.AdminUser{TenantID: 31, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&actor).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/collect/browser-profiles", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	c.Set(ctxkey.TenantID, actor.TenantID)

	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_profile_audit", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("forced audit failure"))
		}
	}))

	_, err = service.Create(c, CreateBody{Name: "e2e profile", Domain: "example.test"}, &actor.ID)
	require.Error(t, err, "profile creation must fail when its audit record cannot be persisted")
	var count int64
	require.NoError(t, db.Model(&CollectBrowserProfile{}).Count(&count).Error)
	require.Zero(t, count, "profile row must roll back with its audit")
}

func TestOpenLoginDoesNotCallCollectorWhenAuditCannotBePersisted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &CollectBrowserProfile{}, &operationlog.OperationLog{}))
	actor := admin.AdminUser{TenantID: 31, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&actor).Error)
	profile := CollectBrowserProfile{TenantID: actor.TenantID, Name: "test profile", Domain: "example.test", ProfileKey: "test_profile", Status: StatusActive}
	require.NoError(t, db.Create(&profile).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/collect/browser-profiles/"+profile.ID.String()+"/open-login", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	c.Set(ctxkey.TenantID, actor.TenantID)
	collector := &profileAuditCollector{}
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}, Collector: collector}
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_profile_open_audit", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("forced audit failure"))
		}
	}))

	_, err = service.OpenLogin(c, profile.ID, URLBody{URL: "https://example.test/login"}, &actor.ID)
	require.ErrorIs(t, err, ErrAuditFailure)
	require.Zero(t, collector.loginCalls, "collector must not be called until the request audit is durable")
}

func TestProfileLifecycleRollsBackWhenAuditCannotBePersisted(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     string
		mutate     func(*Service, *gin.Context, uuid.UUID, *uuid.UUID) error
		wantStatus string
		wantExists bool
	}{
		{name: "disable", status: StatusActive, mutate: (*Service).Disable, wantStatus: StatusActive, wantExists: true},
		{name: "enable", status: StatusDisabled, mutate: (*Service).Enable, wantStatus: StatusDisabled, wantExists: true},
		{name: "delete", status: StatusActive, mutate: (*Service).Delete, wantExists: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &CollectBrowserProfile{}, &operationlog.OperationLog{}))
			actor := admin.AdminUser{TenantID: 31, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
			require.NoError(t, db.Create(&actor).Error)
			profile := CollectBrowserProfile{TenantID: actor.TenantID, Name: "test profile", Domain: "example.test", ProfileKey: "test_profile", Status: tc.status}
			require.NoError(t, db.Create(&profile).Error)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/collect/browser-profiles/"+profile.ID.String(), nil)
			c.Set(ctxkey.AdminID, actor.ID.String())
			c.Set(ctxkey.TenantID, actor.TenantID)
			service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_profile_lifecycle_audit", func(tx *gorm.DB) {
				if tx.Statement != nil && tx.Statement.Table == "operation_logs" {
					tx.AddError(errors.New("forced audit failure"))
				}
			}))

			err = tc.mutate(service, c, profile.ID, &actor.ID)
			require.ErrorIs(t, err, ErrAuditFailure)
			var stored CollectBrowserProfile
			err = db.First(&stored, "id = ?", profile.ID).Error
			if tc.wantExists {
				require.NoError(t, err)
				if tc.wantStatus != "" {
					require.Equal(t, tc.wantStatus, stored.Status)
				}
			} else {
				require.ErrorIs(t, err, gorm.ErrRecordNotFound)
			}
		})
	}
}

func TestProfileCheckRemovesCredentialsFromStoredURL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &CollectBrowserProfile{}, &operationlog.OperationLog{}))
	actor := admin.AdminUser{TenantID: 31, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&actor).Error)
	profile := CollectBrowserProfile{TenantID: actor.TenantID, Name: "test profile", Domain: "example.test", ProfileKey: "test_profile", Status: StatusActive}
	require.NoError(t, db.Create(&profile).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/collect/browser-profiles/"+profile.ID.String()+"/check", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	c.Set(ctxkey.TenantID, actor.TenantID)
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}, Collector: &profileAuditCollector{}}

	_, err = service.Check(c, profile.ID, URLBody{URL: "https://user:pass@example.test/product?access_token=secret#session"}, &actor.ID)
	require.NoError(t, err)
	var stored CollectBrowserProfile
	require.NoError(t, db.First(&stored, "id = ?", profile.ID).Error)
	require.Equal(t, "https://example.test/product", stored.LastCheckURL)
	require.NotContains(t, stored.LastCheckURL, "secret")
	require.NotContains(t, stored.LastCheckURL, "pass")
}

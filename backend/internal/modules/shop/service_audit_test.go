package shop

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

func TestShopMutationsRollbackWhenAuditCannotBePersisted(t *testing.T) {
	db, actor, c := newShopAuditFixture(t, false)
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}

	t.Run("create", func(t *testing.T) {
		_, err := service.Create(c, CreateBody{Platform: "manual", ShopName: "must roll back"}, &actor.ID)
		require.Error(t, err)
		var count int64
		require.NoError(t, db.Model(&Shop{}).Count(&count).Error)
		require.Zero(t, count)
	})

	shopRow := Shop{TenantID: actor.TenantID, Platform: "manual", ShopName: "before", Status: StatusActive, AuthStatus: AuthUnauthorized}
	require.NoError(t, db.Create(&shopRow).Error)
	t.Run("update", func(t *testing.T) {
		_, err := service.Update(c, shopRow.ID, UpdateBody{ShopName: "after"}, &actor.ID)
		require.Error(t, err)
		var stored Shop
		require.NoError(t, db.First(&stored, "id = ?", shopRow.ID).Error)
		require.Equal(t, "before", stored.ShopName)
	})

	t.Run("delete", func(t *testing.T) {
		err := service.Delete(c, shopRow.ID, &actor.ID)
		require.Error(t, err)
		var count int64
		require.NoError(t, db.Model(&Shop{}).Where("id = ?", shopRow.ID).Count(&count).Error)
		require.EqualValues(t, 1, count)
	})

	mockShop := Shop{TenantID: actor.TenantID, Platform: "mock", ShopName: "credentials", Status: StatusActive, AuthStatus: AuthUnauthorized}
	require.NoError(t, db.Create(&mockShop).Error)
	t.Run("credential update", func(t *testing.T) {
		_, err := service.UpdateAuth(c, mockShop.ID, UpdateAuthBody{AppKey: "must roll back"}, &actor.ID)
		require.Error(t, err)
		var stored Shop
		require.NoError(t, db.First(&stored, "id = ?", mockShop.ID).Error)
		require.Equal(t, AuthUnauthorized, stored.AuthStatus)
		var tokenCount int64
		require.NoError(t, db.Model(&ShopAuthToken{}).Where("shop_id = ?", mockShop.ID).Count(&tokenCount).Error)
		require.Zero(t, tokenCount)
	})
}

func TestShopMutationsWriteTraceableAuditRows(t *testing.T) {
	db, actor, c := newShopAuditFixture(t, true)
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}

	created, err := service.Create(c, CreateBody{Platform: "manual", ShopName: "audit me"}, &actor.ID)
	require.NoError(t, err)
	_, err = service.Update(c, created.ID, UpdateBody{ShopName: "updated"}, &actor.ID)
	require.NoError(t, err)

	mockShop := Shop{TenantID: actor.TenantID, Platform: "mock", ShopName: "credentials", Status: StatusActive, AuthStatus: AuthUnauthorized}
	require.NoError(t, db.Create(&mockShop).Error)
	_, err = service.UpdateAuth(c, mockShop.ID, UpdateAuthBody{AppKey: "public-client-id"}, &actor.ID)
	require.NoError(t, err)
	require.NoError(t, service.Delete(c, created.ID, &actor.ID))

	var rows []operationlog.OperationLog
	require.NoError(t, db.Order("action asc").Find(&rows).Error)
	require.Len(t, rows, 4)
	got := make(map[string]operationlog.OperationLog, len(rows))
	for _, row := range rows {
		got[row.Action] = row
		require.Equal(t, actor.ID, *row.AdminUserID)
		require.Equal(t, actor.TenantID, row.TenantID)
		require.Equal(t, adminperm.RoleAdmin, row.AdminRole)
		require.Equal(t, "shop", row.Resource)
	}
	require.Equal(t, created.ID.String(), got["shop.create"].ResourceID)
	require.Equal(t, created.ID.String(), got["shop.update"].ResourceID)
	require.Equal(t, mockShop.ID.String(), got["shop.auth.update"].ResourceID)
	require.Equal(t, created.ID.String(), got["shop.delete"].ResourceID)
}

type auditSettingsProvider struct{ platformp.Provider }

func (auditSettingsProvider) Platform() string { return "e2e_audit_settings" }
func (auditSettingsProvider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.PlatformAppConfigSchema{GroupKey: "platform_e2e_audit", Fields: []platformp.AppConfigField{
		{Name: "client_id", Type: "text"}, {Name: "client_secret", Type: "password", Sensitive: true},
	}}
}
func (auditSettingsProvider) PublishConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.PlatformAppConfigSchema{GroupKey: "platform_publish_e2e_audit", Fields: []platformp.AppConfigField{
		{Name: "warehouse_id", Type: "text"},
	}}
}

func TestPlatformSettingsRollbackWhenAuditCannotBePersisted(t *testing.T) {
	for _, kind := range []string{"application", "publish"} {
		for _, failure := range []string{"missing service", "insert failure"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				db, _, c := newShopAuditFixture(t, true)
				require.NoError(t, db.AutoMigrate(&settings.Setting{}))
				platformp.Register(auditSettingsProvider{Provider: platformp.MustGet("mock")})
				enc, err := encrypt.NewService("platform-settings-test-only")
				require.NoError(t, err)
				service := &Service{DB: db, Settings: &settings.Service{DB: db, Encrypter: enc}, OpLog: &operationlog.Service{DB: db}}
				if failure == "missing service" {
					service.OpLog = nil
				} else {
					require.NoError(t, db.Exec("CREATE TRIGGER reject_platform_audit BEFORE INSERT ON operation_logs BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END").Error)
				}
				if kind == "application" {
					_, err = service.PutPlatformAppSettings(c, "e2e_audit_settings", map[string]interface{}{"client_id": "must roll back", "client_secret": "e2e-secret"})
				} else {
					_, err = service.PutPlatformPublishSettings(c, "e2e_audit_settings", map[string]interface{}{"warehouse_id": "must roll back"})
				}
				require.Error(t, err, "sensitive settings must fail closed without durable audit")
				var count int64
				require.NoError(t, db.Model(&settings.Setting{}).Count(&count).Error)
				require.Zero(t, count, "audit failure must roll back every settings field")
			})
		}
	}
}

func TestPlatformSettingsCommitWithRedactedAudit(t *testing.T) {
	db, actor, c := newShopAuditFixture(t, true)
	require.NoError(t, db.AutoMigrate(&settings.Setting{}))
	platformp.Register(auditSettingsProvider{Provider: platformp.MustGet("mock")})
	enc, err := encrypt.NewService("platform-settings-test-only")
	require.NoError(t, err)
	service := &Service{DB: db, Settings: &settings.Service{DB: db, Encrypter: enc}, OpLog: &operationlog.Service{DB: db}}
	result, err := service.PutPlatformAppSettings(c, "e2e_audit_settings", map[string]interface{}{"client_id": "e2e-client", "client_secret": "e2e-private-secret"})
	require.NoError(t, err)
	require.Equal(t, "****", result.Values["client_secret"])
	_, err = service.PutPlatformPublishSettings(c, "e2e_audit_settings", map[string]interface{}{"warehouse_id": "e2e-warehouse"})
	require.NoError(t, err)
	var rows []operationlog.OperationLog
	require.NoError(t, db.Order("created_at").Find(&rows).Error)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, actor.ID, *row.AdminUserID)
		require.NotEmpty(t, row.EntryHash)
		require.NotContains(t, row.Message, "e2e-private-secret")
	}
	var stored settings.Setting
	require.NoError(t, db.Where("item_key = ?", "client_secret").First(&stored).Error)
	require.NotContains(t, stored.ItemValue, "e2e-private-secret")
}

func newShopAuditFixture(t *testing.T, withAudit bool) (*gorm.DB, admin.AdminUser, *gin.Context) {
	t.Helper()
	platformp.Bootstrap()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	models := []any{&admin.AdminUser{}, &admin.UserStorePermission{}, &Shop{}, &ShopAuthToken{}}
	if withAudit {
		models = append(models, &operationlog.OperationLog{})
	}
	require.NoError(t, db.AutoMigrate(models...))
	actor := admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     45,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test-only",
		Role:         adminperm.RoleAdmin,
		Status:       admin.StatusActive,
		TokenVersion: 1,
	}
	require.NoError(t, db.Create(&actor).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/shops", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	c.Set(ctxkey.TenantID, actor.TenantID)
	return db, actor, c
}

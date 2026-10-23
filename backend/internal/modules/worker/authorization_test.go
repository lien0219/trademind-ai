package worker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

type monitorTaskFixture struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey"`
	TenantID    int64
	ShopID      uuid.UUID `gorm:"type:char(36)"`
	CreatedBy   uuid.UUID `gorm:"type:char(36)"`
	Status      string
	LockedBy    *string
	LockedUntil *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func TestMonitorEnforcesPermissionTenantAndShopScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:monitor_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &Instance{}))
	require.NoError(t, db.Exec("CREATE TABLE shops (id char(36) PRIMARY KEY, tenant_id bigint NOT NULL)").Error)
	actor := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(), Role: adminperm.RoleOperator, Status: admin.StatusActive, PasswordHash: "test-only"}
	readonly := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(), Role: adminperm.RoleReadonly, Status: admin.StatusActive, PasswordHash: "test-only"}
	tenantAdmin := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive, PasswordHash: "test-only"}
	root := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive, PasswordHash: "test-only"}
	for _, u := range []*admin.AdminUser{&actor, &readonly, &tenantAdmin, &root} {
		require.NoError(t, db.Create(u).Error)
	}
	allowedShop := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO shops (id, tenant_id) VALUES (?, ?)", allowedShop, actor.TenantID).Error)
	require.NoError(t, db.Create(&admin.UserStorePermission{UserID: actor.ID, StoreID: allowedShop, PermissionScope: "operate"}).Error)
	lockOwner := "e2e-worker"
	allowedTask := uuid.New()
	for _, table := range []string{"collect_tasks", "image_tasks", "order_sync_tasks", "customer_message_sync_tasks", "product_publish_tasks", "inventory_sync_tasks"} {
		require.NoError(t, db.Table(table).AutoMigrate(&monitorTaskFixture{}))
		for _, row := range []monitorTaskFixture{
			{ID: allowedTask, TenantID: 7, ShopID: allowedShop, CreatedBy: actor.ID, Status: "running", LockedBy: &lockOwner},
			{ID: uuid.New(), TenantID: 7, ShopID: uuid.New(), CreatedBy: tenantAdmin.ID, Status: "running", LockedBy: &lockOwner},
			{ID: uuid.New(), TenantID: 8, ShopID: allowedShop, CreatedBy: actor.ID, Status: "running", LockedBy: &lockOwner},
		} {
			require.NoError(t, db.Table(table).Create(&row).Error)
		}
	}
	call := func(actor admin.AdminUser) *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set(ctxkey.AdminID, actor.ID.String()); c.Set(ctxkey.TenantID, actor.TenantID) })
		Register(r.Group("/api/v1"), &Handler{DB: db})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/workers/monitor", nil))
		return w
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workers/monitor", nil)
	ctx.Set(ctxkey.AdminID, actor.ID.String())
	ctx.Set(ctxkey.TenantID, actor.TenantID)
	p, err := adminperm.LoadPrincipal(ctx, db)
	require.NoError(t, err)
	t.Logf("operator principal: role=%s perms=%v stores=%v", p.Role, p.Permissions, p.AllowedStoreIDs())
	t.Run("readonly denied", func(t *testing.T) {
		require.Equal(t, http.StatusForbidden, call(readonly).Code)
	})
	for _, actorCase := range []struct {
		name  string
		actor admin.AdminUser
		count int
	}{
		{"operator only own or granted tasks", actor, 1},
		{"administrator only own tenant", tenantAdmin, 2},
		{"root cannot read other tenant leases", root, 0},
	} {
		t.Run(actorCase.name, func(t *testing.T) {
			w := call(actorCase.actor)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var envelope struct {
				Data MonitorResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
			for kind, rows := range envelope.Data.LeasedTasks {
				require.Len(t, rows, actorCase.count, kind)
				if actorCase.count == 1 {
					require.Equal(t, allowedTask, rows[0].ID, kind)
				}
			}
		})
	}
}

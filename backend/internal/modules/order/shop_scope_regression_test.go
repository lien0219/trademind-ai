package order

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/tenantconfig"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestOrderShopReferencesRejectUnownedAndCrossTenantShops(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_shop_scope_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&shop.Shop{}, &Order{}, &OrderItem{}, &OrderShipment{}, &tenantconfig.Setting{}))

	allowed := shop.Shop{TenantID: 7, Platform: "manual", ShopName: "allowed", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	ungranted := shop.Shop{TenantID: 7, Platform: "manual", ShopName: "ungranted", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	foreign := shop.Shop{TenantID: 8, Platform: "manual", ShopName: "foreign", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	require.NoError(t, db.Create(&allowed).Error)
	require.NoError(t, db.Create(&ungranted).Error)
	require.NoError(t, db.Create(&foreign).Error)

	actorID := uuid.New()
	c := orderShopScopeContext(7, actorID, adminperm.RoleOperator, []uuid.UUID{allowed.ID})
	svc := &Service{DB: db, Shops: &shop.Service{DB: db}}

	for _, tc := range []struct {
		name string
		shop uuid.UUID
	}{
		{name: "same tenant without grant", shop: ungranted.ID},
		{name: "different tenant", shop: foreign.ID},
	} {
		t.Run("create "+tc.name, func(t *testing.T) {
			_, err := svc.Create(c, CreateBody{
				Platform:     "manual",
				ShopID:       &tc.shop,
				OrderNo:      "scope-create-" + tc.name,
				CustomerName: "buyer",
			}, &actorID)
			require.Error(t, err)
		})
	}

	existing := Order{
		Base: model.Base{ID: uuid.New()}, TenantID: 7, Platform: "manual", ShopID: &allowed.ID,
		OrderNo: "scope-update", CustomerName: "buyer", Status: StatusPending,
		PaymentStatus: PaymentUnpaid, FulfillmentStatus: FulfillmentUnfulfilled, Currency: "USD",
	}
	require.NoError(t, db.Create(&existing).Error)
	for _, tc := range []struct {
		name string
		shop uuid.UUID
	}{
		{name: "same tenant without grant", shop: ungranted.ID},
		{name: "different tenant", shop: foreign.ID},
	} {
		t.Run("update "+tc.name, func(t *testing.T) {
			_, err := svc.Update(c, existing.ID, UpdateBody{ShopID: &tc.shop}, &actorID)
			require.Error(t, err)
			var row Order
			require.NoError(t, db.First(&row, "id = ?", existing.ID).Error)
			require.NotNil(t, row.ShopID)
			require.Equal(t, allowed.ID, *row.ShopID)
		})
	}
}

func orderShopScopeContext(tenantID int64, userID uuid.UUID, role string, storeIDs []uuid.UUID) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	c.Set(ctxkey.AdminID, userID.String())
	grants := make([]adminperm.StoreGrant, 0, len(storeIDs))
	for _, storeID := range storeIDs {
		grants = append(grants, adminperm.StoreGrant{StoreID: storeID, PermissionScope: admin.StorePermScopeOperate})
	}
	c.Set("adminperm.principal", &adminperm.Principal{
		UserID: userID, Role: role, Permissions: adminperm.PermissionsForRole(role), StoreGrants: grants,
	})
	return c
}

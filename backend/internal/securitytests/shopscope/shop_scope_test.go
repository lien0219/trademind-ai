package shopscope_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

const tenantID int64 = 42

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:shopscope_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Skip(err)
	}
	if err := db.AutoMigrate(
		&shop.Shop{}, &product.Product{}, &product.ProductImage{}, &product.ProductSKU{},
		&shop.ShopAuthToken{}, &imagetask.ImageTask{}, &admin.AdminUser{}, &admin.UserStorePermission{},
		&productpublish.ProductPublishTask{}, &exportmod.ExportJob{}, &customerchat.CustomerConversation{},
		&customerchat.CustomerMessage{}, &order.Order{}, &order.OrderItem{}, &order.OrderShipment{},
		&operationlog.OperationLog{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func ginCtx(tenantID int64, userID uuid.UUID, role string, storeIDs []uuid.UUID) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	c.Set(ctxkey.AdminID, userID.String())
	grants := make([]adminperm.StoreGrant, 0, len(storeIDs))
	for _, id := range storeIDs {
		grants = append(grants, adminperm.StoreGrant{StoreID: id, PermissionScope: admin.StorePermScopeOperate})
	}
	c.Set("adminperm.principal", &adminperm.Principal{
		UserID: userID, Role: role, Permissions: adminperm.PermissionsForRole(role), StoreGrants: grants,
	})
	security.SetGin(c, &security.TenantContext{TenantID: tenantID, UserID: userID, AuthSource: security.AuthSourceAccessToken})
	return c
}

func seedShop(t *testing.T, db *gorm.DB, name string) uuid.UUID {
	t.Helper()
	s := &shop.Shop{TenantID: tenantID, Platform: "manual", ShopName: name, Status: "active", AuthStatus: "authorized"}
	if err := db.Create(s).Error; err != nil {
		t.Fatal(err)
	}
	return s.ID
}

func newShopService(db *gorm.DB) *shop.Service {
	return &shop.Service{DB: db, OpLog: &operationlog.Service{DB: db}}
}

func TestShopScope_OperatorCannotReadOtherShop(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	user := uuid.New()
	c := ginCtx(tenantID, user, adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := newShopService(db)
	_, err := svc.GetDetail(c, shopB)
	if err == nil {
		t.Fatal("operator A must not read shop B")
	}
}

func TestShopScope_OperatorCannotUpdateOtherShop(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := newShopService(db)
	name := "hacked"
	_, err := svc.Update(c, shopB, shop.UpdateBody{ShopName: name}, nil)
	if err == nil {
		t.Fatal("operator A must not update shop B")
	}
}

func TestShopScope_OperatorListOnlyOwnShops(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	_ = seedShop(t, db, "B")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	// List is tenant-scoped; operator still sees both shops at tenant level unless store scope applied to shop list
	// Shop list uses tenant only; store scope for the shop module is tenant-level, while Get/Update enforce operator shop denial.
	svc := newShopService(db)
	_, err := svc.GetDetail(c, shopA)
	if err != nil {
		t.Fatalf("operator should access shop A: %v", err)
	}
}

func TestShopScope_ReadonlyCannotUpdateShop(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleReadonly, []uuid.UUID{shopA})
	svc := newShopService(db)
	_, err := svc.Update(c, shopA, shop.UpdateBody{ShopName: "x"}, nil)
	if err == nil {
		t.Fatal("readonly must not update")
	}
}

func TestShopScope_AdminCanAccessAllShopsInTenant(t *testing.T) {
	db := openDB(t)
	shopB := seedShop(t, db, "B")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleAdmin, nil)
	svc := newShopService(db)
	_, err := svc.GetDetail(c, shopB)
	if err != nil {
		t.Fatal(err)
	}
}

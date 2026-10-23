package shopscope_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func seedProductPublishTask(t *testing.T, db *gorm.DB, shopID, productID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	row := &productpublish.ProductPublishTask{
		TenantID: tenantID, ShopID: shopID, ProductID: productID, TargetStoreID: shopID,
		Platform: "manual", TaskType: "publish", Status: productpublish.TaskFailed,
		Mode: "manual", Title: title,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedProductForShop(t *testing.T, db *gorm.DB, title string) uuid.UUID {
	t.Helper()
	p := &product.Product{TenantID: tenantID, Title: title, OriginalTitle: title, Status: product.StatusDraft, Currency: "CNY", Source: "test"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func seedOrderForShop(t *testing.T, db *gorm.DB, shopID uuid.UUID, orderNo string) uuid.UUID {
	t.Helper()
	ext := orderNo
	o := &order.Order{TenantID: tenantID, ShopID: &shopID, Platform: "manual", ExternalOrderID: &ext, OrderNo: orderNo, Status: order.StatusPaid, Currency: "CNY"}
	if err := db.Create(o).Error; err != nil {
		t.Fatal(err)
	}
	return o.ID
}

func seedConversation(t *testing.T, db *gorm.DB, shopID uuid.UUID, customer string) uuid.UUID {
	t.Helper()
	row := &customerchat.CustomerConversation{
		TenantID: tenantID, Platform: "manual", ShopID: &shopID,
		CustomerName: customer, CustomerLanguage: "en", Status: customerchat.StatusOpen,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedExportJob(t *testing.T, db *gorm.DB, shopID uuid.UUID) uuid.UUID {
	t.Helper()
	row := &exportmod.ExportJob{
		TenantID: tenantID, ExportType: exportmod.ExportTypeOrders, Status: exportmod.ExportStatusPending,
		ShopID: &shopID, MaskedPII: true,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedOpLog(t *testing.T, db *gorm.DB, shopID uuid.UUID, message string) {
	t.Helper()
	row := &operationlog.OperationLog{
		TenantID: tenantID, ShopID: &shopID, Username: "op", Action: "test", Resource: "shop",
		Method: "GET", Path: "/test", Status: "success", Message: message,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
}

// --- Extended shop scope (15 cases) ---

func TestShopScope_ProductPublishListOnlyGrantedShops(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	pidA := seedProductForShop(t, db, "pa")
	pidB := seedProductForShop(t, db, "pb")
	seedProductPublishTask(t, db, shopA, pidA, "task-a")
	seedProductPublishTask(t, db, shopB, pidB, "task-b-secret")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &productpublish.Service{DB: db}
	res, err := svc.ListTasks(c, productpublish.ListTasksQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("operator should see 1 publish task, got %d", res.Total)
	}
	if res.Items[0].ShopID != shopA {
		t.Fatal("operator must not see other shop publish tasks")
	}
}

func TestShopScope_ExportCreateOtherShopDenied(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &exportmod.Service{DB: db}
	_, err := svc.CreateJob(c, exportmod.CreateJobInput{ExportType: exportmod.ExportTypeOrders, ShopID: &shopB, MaskedPII: true})
	if err == nil {
		t.Fatal("operator must not create export for shop B")
	}
}

func TestShopScope_ExportGetOtherShopDenied(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	jid := seedExportJob(t, db, shopB)
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &exportmod.Service{DB: db}
	_, err := svc.GetJob(c, jid)
	if err == nil {
		t.Fatal("operator must not read export job for shop B")
	}
}

func TestShopScope_ExportListOnlyGrantedShops(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	seedExportJob(t, db, shopA)
	seedExportJob(t, db, shopB)
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &exportmod.Service{DB: db}
	rows, total, err := svc.ListJobs(c, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected 1 export job, got total=%d len=%d", total, len(rows))
	}
}

func TestShopScope_CustomerChatGetOtherShopDenied(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	cid := seedConversation(t, db, shopB, "secret-customer")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &customerchat.Service{DB: db}
	_, err := svc.GetConversation(c, cid)
	if err == nil {
		t.Fatal("operator must not read conversation in shop B")
	}
}

func TestShopScope_CustomerChatListOnlyGrantedShops(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	seedConversation(t, db, shopA, "alice")
	seedConversation(t, db, shopB, "secret-bob")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	tx := db.WithContext(c.Request.Context()).Model(&customerchat.CustomerConversation{})
	scoped, err := adminperm.ApplyStoreScope(c, db, tx, "shop_id")
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	if err := scoped.Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("expected 1 conversation, got %d", total)
	}
}

func TestShopScope_OrderGetOtherShopDenied(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	oid := seedOrderForShop(t, db, shopB, "ORD-B-001")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &order.Service{DB: db}
	_, err := svc.Get(c, oid)
	if err == nil {
		t.Fatal("operator must not read order in shop B")
	}
}

func TestShopScope_OrderListOnlyGrantedShops(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	seedOrderForShop(t, db, shopA, "ORD-A-001")
	seedOrderForShop(t, db, shopB, "ORD-B-001")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &order.Service{DB: db}
	res, err := svc.List(c, order.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 order, got %d", res.Total)
	}
}

func TestShopScope_OperatorMultiShopCanAccessBoth(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA, shopB})
	svc := newShopService(db)
	if _, err := svc.GetDetail(c, shopA); err != nil {
		t.Fatalf("operator should access shop A: %v", err)
	}
	if _, err := svc.GetDetail(c, shopB); err != nil {
		t.Fatalf("operator should access shop B: %v", err)
	}
}

func TestShopScope_OperatorNoGrantsCannotGetShop(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, nil)
	svc := newShopService(db)
	_, err := svc.GetDetail(c, shopA)
	if err == nil {
		t.Fatal("operator without grants must not read shop")
	}
}

func TestShopScope_ReadonlyCanGetCannotUpdate(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleReadonly, []uuid.UUID{shopA})
	svc := newShopService(db)
	if _, err := svc.GetDetail(c, shopA); err != nil {
		t.Fatalf("readonly should read shop A: %v", err)
	}
	_, err := svc.Update(c, shopA, shop.UpdateBody{ShopName: "x"}, nil)
	if err == nil {
		t.Fatal("readonly must not update")
	}
}

func TestShopScope_AdminCanAccessExportOtherShop(t *testing.T) {
	db := openDB(t)
	shopB := seedShop(t, db, "B")
	jid := seedExportJob(t, db, shopB)
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleAdmin, nil)
	svc := &exportmod.Service{DB: db}
	if _, err := svc.GetJob(c, jid); err != nil {
		t.Fatal(err)
	}
}

func TestShopScope_OperatorCanCreateExportOwnShop(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &exportmod.Service{DB: db}
	row, err := svc.CreateJob(c, exportmod.CreateJobInput{ExportType: exportmod.ExportTypeOrders, ShopID: &shopA, MaskedPII: true})
	if err != nil {
		t.Fatal(err)
	}
	if row.ShopID == nil || *row.ShopID != shopA {
		t.Fatal("expected export job for shop A")
	}
}

func TestShopScope_OpLogListStoreScoped(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	seedOpLog(t, db, shopA, "log-a")
	seedOpLog(t, db, shopB, "log-b-secret")
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleOperator, []uuid.UUID{shopA})
	svc := &operationlog.Service{DB: db}
	res, err := svc.List(c, operationlog.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 op log, got %d", res.Total)
	}
}

func TestShopScope_AdminCanUpdateAnyShopInTenant(t *testing.T) {
	db := openDB(t)
	shopB := seedShop(t, db, "B")
	actorID := uuid.New()
	actor := admin.AdminUser{
		Base:         model.Base{ID: actorID},
		TenantID:     tenantID,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test-only",
		Role:         adminperm.RoleAdmin,
		Status:       admin.StatusActive,
	}
	if err := db.Create(&actor).Error; err != nil {
		t.Fatal(err)
	}
	c := ginCtx(tenantID, actorID, adminperm.RoleAdmin, nil)
	svc := newShopService(db)
	name := "admin-updated"
	_, err := svc.Update(c, shopB, shop.UpdateBody{ShopName: name}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestShopScope_ExportAdminListSeesAllShops(t *testing.T) {
	db := openDB(t)
	shopA := seedShop(t, db, "A")
	shopB := seedShop(t, db, "B")
	seedExportJob(t, db, shopA)
	seedExportJob(t, db, shopB)
	c := ginCtx(tenantID, uuid.New(), adminperm.RoleAdmin, nil)
	svc := &exportmod.Service{DB: db}
	_, total, err := svc.ListJobs(c, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("admin should see 2 export jobs, got %d", total)
	}
}

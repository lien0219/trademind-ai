package idor_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

const tenantA int64 = 1001
const tenantB int64 = 2002

func openIDORDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:idor_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&product.Product{}, &product.ProductImage{}, &product.ProductSKU{},
		&product.ProductPlatformPublishConfig{}, &product.ProductAIContentApplication{}, &aitask.AITask{},
		&order.Order{}, &shop.Shop{}, &files.FileRecord{}, &admin.AdminUser{},
		&imagetask.ImageTask{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func ginWithTenant(tenantID int64, userID uuid.UUID) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	c.Set(ctxkey.AdminID, userID.String())
	c.Set("adminperm.principal", &adminperm.Principal{
		UserID:      userID,
		Role:        adminperm.RoleAdmin,
		Permissions: adminperm.PermissionsForRole(adminperm.RoleAdmin),
	})
	security.SetGin(c, &security.TenantContext{TenantID: tenantID, UserID: userID, AuthSource: security.AuthSourceAccessToken})
	return c
}

func seedProduct(t *testing.T, db *gorm.DB, tenantID int64, title string) uuid.UUID {
	t.Helper()
	p := &product.Product{TenantID: tenantID, Title: title, OriginalTitle: title, Status: product.StatusDraft, Currency: "CNY", Source: "test"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func seedOrder(t *testing.T, db *gorm.DB, tenantID int64, shopID uuid.UUID) uuid.UUID {
	t.Helper()
	ext := uuid.NewString()
	o := &order.Order{TenantID: tenantID, ShopID: &shopID, Platform: "douyin_shop", ExternalOrderID: &ext, OrderNo: ext, Status: order.StatusPaid, Currency: "CNY"}
	if err := db.Create(o).Error; err != nil {
		t.Fatal(err)
	}
	return o.ID
}

func seedShop(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	s := &shop.Shop{TenantID: tenantID, Platform: "douyin_shop", ShopName: "shop", Status: "active"}
	if err := db.Create(s).Error; err != nil {
		t.Fatal(err)
	}
	return s.ID
}

func seedFile(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	f := &files.FileRecord{
		TenantID: tenantID, OriginalName: "a.jpg", ObjectKey: fmt.Sprintf("t%d/%s.jpg", tenantID, uuid.NewString()),
		PublicURL: "http://example/x.jpg", ContentType: "image/jpeg", Size: 10, StorageKind: "local",
		SecurityStatus: files.SecurityClean, ScanStatus: files.SecurityClean,
	}
	if err := db.Create(f).Error; err != nil {
		t.Fatal(err)
	}
	return f.ID
}

// --- Product IDOR (4 cases) ---

func TestIDOR_ProductGetCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	userA := uuid.New()
	pidB := seedProduct(t, db, tenantB, "secret-b")
	c := ginWithTenant(tenantA, userA)
	_, err := svc.Get(c, pidB)
	if err == nil {
		t.Fatal("expected cross-tenant get denied")
	}
}

func TestIDOR_ProductUpdateCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	pidB := seedProduct(t, db, tenantB, "secret-b")
	title := "hacked"
	c := ginWithTenant(tenantA, uuid.New())
	_, err := svc.Update(c, pidB, product.UpdateBody{Title: &title}, nil)
	if err == nil {
		t.Fatal("expected cross-tenant update denied")
	}
	var p product.Product
	db.First(&p, "id = ?", pidB)
	if p.Title == "hacked" {
		t.Fatal("cross-tenant update mutated data")
	}
}

func TestIDOR_ProductDeleteCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	pidB := seedProduct(t, db, tenantB, "secret-b")
	c := ginWithTenant(tenantA, uuid.New())
	err := svc.Delete(c, pidB, nil)
	if err == nil {
		t.Fatal("expected cross-tenant delete denied")
	}
	var count int64
	db.Model(&product.Product{}).Where("id = ?", pidB).Count(&count)
	if count != 1 {
		t.Fatal("cross-tenant delete removed row")
	}
}

func TestIDOR_ProductListExcludesOtherTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	seedProduct(t, db, tenantA, "a")
	seedProduct(t, db, tenantB, "b")
	c := ginWithTenant(tenantA, uuid.New())
	res, err := svc.List(c, product.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 product, got %d", res.Total)
	}
}

func TestIDOR_ProductImagesRejectCrossTenantWrites(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	productB := seedProduct(t, db, tenantB, "secret-image-product")
	image := &product.ProductImage{
		ProductID: productB,
		ImageType: product.ImageTypeMain,
		PublicURL: "https://tenant-b.example/image.jpg",
	}
	if err := db.Create(image).Error; err != nil {
		t.Fatal(err)
	}
	fileB := seedFile(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())

	if _, err := svc.CreateProductImage(c, productB, product.ImageCreateBody{FileID: &fileB, ImageType: product.ImageTypeMain}, nil); err == nil {
		t.Fatal("expected cross-tenant image create denied")
	}
	if _, err := svc.UpdateProductImage(c, productB, image.ID, product.ImageUpdateBody{}, nil); err == nil {
		t.Fatal("expected cross-tenant image update denied")
	}
	if err := svc.DeleteProductImage(c, productB, image.ID, nil); err == nil {
		t.Fatal("expected cross-tenant image delete denied")
	}
	if err := svc.ReorderProductImages(c, productB, product.ImageReorderBody{ImageIDs: []uuid.UUID{image.ID}}, nil); err == nil {
		t.Fatal("expected cross-tenant image reorder denied")
	}

	var persisted product.ProductImage
	if err := db.First(&persisted, "id = ?", image.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.PublicURL != image.PublicURL {
		t.Fatal("cross-tenant image write mutated data")
	}
}

func TestIDOR_ProductPlatformConfigCrossTenantReadWriteDenied(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	productB := seedProduct(t, db, tenantB, "tenant-b-config")
	shopB := seedShop(t, db, tenantB)
	if err := db.Create(&product.ProductPlatformPublishConfig{
		ProductID: productB, Platform: "douyin_shop", ShopID: &shopB,
		CategoryID: "leaf-b",
	}).Error; err != nil {
		t.Fatal(err)
	}
	c := ginWithTenant(tenantA, uuid.New())
	if _, err := svc.GetPlatformPublishConfig(c, productB, "douyin_shop"); err == nil {
		t.Fatal("expected cross-tenant platform config read denied")
	}
	if _, err := svc.PutPlatformPublishConfig(c, productB, "douyin_shop", product.PlatformPublishConfigBody{ShopID: shopB.String()}, nil); err == nil {
		t.Fatal("expected cross-tenant platform config write denied")
	}

	productA := seedProduct(t, db, tenantA, "tenant-a-config")
	if err := db.Create(&product.ProductPlatformPublishConfig{
		ProductID: productA, Platform: "douyin_shop", ShopID: &shopB,
		CategoryID: "leaf-b",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetPlatformPublishConfig(c, productA, "douyin_shop"); err == nil {
		t.Fatal("expected persisted cross-tenant shop reference to be denied on read")
	}
}

func TestIDOR_ProductDouyinMappingRejectsCrossTenantShop(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	productA := seedProduct(t, db, tenantA, "tenant-a-mapping")
	shopB := seedShop(t, db, tenantB)
	ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: tenantA})
	err := svc.SaveDouyinDraftMapping(ctx, productA, &product.DouyinDraftMapping{ShopID: shopB.String()})
	if err == nil {
		t.Fatal("expected cross-tenant Douyin mapping shop to be rejected")
	}
	var count int64
	if err := db.Model(&product.ProductPlatformPublishConfig{}).Where("product_id = ?", productA).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("cross-tenant mapping must not create config, got %d", count)
	}
}

func TestIDOR_ProductOperationProgressCrossTenantDenied(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	productB := seedProduct(t, db, tenantB, "tenant-b-progress")
	ctx := security.WithTenantContext(context.Background(), security.WorkerTenantContext(tenantA, uuid.Nil))
	if _, err := svc.GetOperationProgress(ctx, productB); err == nil {
		t.Fatal("expected cross-tenant operation progress denied")
	}
}

func TestIDOR_ProductAITasksCrossTenantDenied(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db, AITasks: &aitask.Service{DB: db}}
	productB := seedProduct(t, db, tenantB, "tenant-b-ai-task")
	task := &aitask.AITask{TaskType: "product_title_optimize", Status: aitask.StatusSuccess, ProductID: &productB}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	c := ginWithTenant(tenantA, uuid.New())
	if _, err := svc.ListRecentAITasks(c, productB, 10); err == nil {
		t.Fatal("expected cross-tenant AI task list denied")
	}
}

func TestIDOR_ProductAIApplyCrossTenantDenied(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db, AITasks: &aitask.Service{DB: db}}
	productB := seedProduct(t, db, tenantB, "tenant-b-ai-apply")
	task := &aitask.AITask{TaskType: "product_title_optimize", Status: aitask.StatusSuccess, ProductID: &productB}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	c := ginWithTenant(tenantA, uuid.New())
	_, err := svc.ApplyAITitle(c, productB, product.ApplyAITitleBody{
		AITitle: "cross-tenant title", TaskID: task.ID.String(),
	}, nil)
	if err == nil {
		t.Fatal("expected cross-tenant AI apply denied")
	}
	var persisted product.Product
	if err := db.First(&persisted, "id = ?", productB).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.AITitle != "" {
		t.Fatal("cross-tenant AI apply mutated product")
	}
}

// --- Order IDOR (3 cases) ---

func TestIDOR_OrderGetCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &order.Service{DB: db}
	shopB := seedShop(t, db, tenantB)
	oid := seedOrder(t, db, tenantB, shopB)
	c := ginWithTenant(tenantA, uuid.New())
	_, err := svc.Get(c, oid)
	if err == nil {
		t.Fatal("expected cross-tenant order get denied")
	}
}

func TestIDOR_OrderPeekCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &order.Service{DB: db}
	shopB := seedShop(t, db, tenantB)
	oid := seedOrder(t, db, tenantB, shopB)
	c := ginWithTenant(tenantA, uuid.New())
	_, err := svc.PeekOrderBeforeUpdate(c, oid)
	if err == nil {
		t.Fatal("expected cross-tenant peek denied")
	}
}

func TestIDOR_OrderListTenantScoped(t *testing.T) {
	db := openIDORDB(t)
	svc := &order.Service{DB: db}
	shopA := seedShop(t, db, tenantA)
	shopB := seedShop(t, db, tenantB)
	seedOrder(t, db, tenantA, shopA)
	seedOrder(t, db, tenantB, shopB)
	c := ginWithTenant(tenantA, uuid.New())
	res, err := svc.List(c, order.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 order, got %d", res.Total)
	}
}

// --- Shop IDOR (3 cases) ---

func TestIDOR_ShopGetCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &shop.Service{DB: db}
	sid := seedShop(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	_, err := svc.GetDetail(c, sid)
	if err == nil {
		t.Fatal("expected cross-tenant shop get denied")
	}
}

func TestIDOR_ShopUpdateCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &shop.Service{DB: db}
	sid := seedShop(t, db, tenantB)
	name := "hacked"
	c := ginWithTenant(tenantA, uuid.New())
	_, err := svc.Update(c, sid, shop.UpdateBody{ShopName: name}, nil)
	if err == nil {
		t.Fatal("expected cross-tenant shop update denied")
	}
}

func TestIDOR_ShopListTenantScoped(t *testing.T) {
	db := openIDORDB(t)
	svc := &shop.Service{DB: db}
	seedShop(t, db, tenantA)
	seedShop(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	res, err := svc.List(c, shop.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 shop, got %d", res.Total)
	}
}

// --- Files IDOR (4 cases) ---

func TestIDOR_FileDeleteCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &files.Service{DB: db}
	fid := seedFile(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	err := svc.DeleteRecordByTenant(c, fid)
	if err == nil {
		t.Fatal("expected cross-tenant file delete denied")
	}
}

func TestIDOR_FileListTenantScoped(t *testing.T) {
	db := openIDORDB(t)
	svc := &files.Service{DB: db}
	seedFile(t, db, tenantA)
	seedFile(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	res, err := svc.List(c, files.ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 file, got %d", res.Total)
	}
}

func TestIDOR_FileAccessCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	access := &files.ObjectAccessService{DB: db, Cfg: testCfg()}
	fid := seedFile(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	_, err := access.CreateDownloadURL(c, fid, "private")
	if err == nil {
		t.Fatal("expected cross-tenant signed url denied")
	}
}

func TestIDOR_FileDownloadLoadCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	access := &files.ObjectAccessService{DB: db, Cfg: testCfg()}
	fid := seedFile(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	_, err := access.LoadForDownload(c.Request.Context(), tenantA, fid)
	if err == nil {
		t.Fatal("expected cross-tenant download load denied")
	}
}

func testCfg() *config.Config {
	return &config.Config{JWTSecret: "test-secret-key-32chars-minimum!!", Tenant: config.TenantConfig{PrivateDownloadURLTTL: 300}}
}

// Additional matrix cases to reach 20+

func TestIDOR_ProductGetSameIDDifferentTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	pidA := seedProduct(t, db, tenantA, "a-only")
	c := ginWithTenant(tenantB, uuid.New())
	_, err := svc.Get(c, pidA)
	if err == nil {
		t.Fatal("expected tenant B cannot read tenant A product")
	}
}

func TestIDOR_OrderUpdateCrossTenantNoMutation(t *testing.T) {
	db := openIDORDB(t)
	svc := &order.Service{DB: db}
	shopB := seedShop(t, db, tenantB)
	oid := seedOrder(t, db, tenantB, shopB)
	c := ginWithTenant(tenantA, uuid.New())
	name := "evil"
	_, err := svc.Update(c, oid, order.UpdateBody{CustomerName: name}, nil)
	if err == nil {
		t.Fatal("expected denied")
	}
}

func TestIDOR_ShopDeleteCrossTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &shop.Service{DB: db}
	sid := seedShop(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	err := svc.Delete(c, sid, nil)
	if err == nil {
		t.Fatal("expected denied")
	}
}

func TestIDOR_MissingTenantContext(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	pid := seedProduct(t, db, tenantA, "a")
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	_, err := svc.Get(c, pid)
	if err == nil {
		t.Fatal("expected missing tenant error")
	}
}

func TestIDOR_FilePendingNotAccessible(t *testing.T) {
	db := openIDORDB(t)
	access := &files.ObjectAccessService{DB: db, Cfg: testCfg()}
	f := &files.FileRecord{
		TenantID: tenantA, OriginalName: "p.jpg", ObjectKey: "t1001/p.jpg", PublicURL: "http://x", ContentType: "image/jpeg",
		Size: 1, StorageKind: "local", SecurityStatus: files.SecurityPendingScan, ScanStatus: files.SecurityPendingScan,
	}
	db.Create(f)
	c := ginWithTenant(tenantA, uuid.New())
	_, err := access.CreateDownloadURL(c, f.ID, "private")
	if err == nil {
		t.Fatal("pending_scan must not be accessible")
	}
}

func TestIDOR_FileQuarantinedNotAccessible(t *testing.T) {
	db := openIDORDB(t)
	access := &files.ObjectAccessService{DB: db, Cfg: testCfg()}
	f := &files.FileRecord{
		TenantID: tenantA, OriginalName: "q.jpg", ObjectKey: "t1001/q.jpg", PublicURL: "http://x", ContentType: "image/jpeg",
		Size: 1, StorageKind: "local", SecurityStatus: files.SecurityQuarantined, ScanStatus: files.SecurityQuarantined,
	}
	db.Create(f)
	c := ginWithTenant(tenantA, uuid.New())
	_, err := access.CreateDownloadURL(c, f.ID, "private")
	if err == nil {
		t.Fatal("quarantined must not be accessible")
	}
}

func TestIDOR_ProductCreateStampsTenant(t *testing.T) {
	db := openIDORDB(t)
	svc := &product.Service{DB: db}
	c := ginWithTenant(tenantA, uuid.New())
	dto, err := svc.Create(c, product.CreateBody{Title: "new", TenantID: tenantB}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dto.TenantID != tenantA {
		t.Fatalf("create must stamp JWT tenant, got %d", dto.TenantID)
	}
}

func TestIDOR_CrossTenantObjectKeyPrefix(t *testing.T) {
	db := openIDORDB(t)
	f := &files.FileRecord{
		TenantID: tenantB, OriginalName: "x.jpg", ObjectKey: "t2002/secret.jpg", PublicURL: "http://x", ContentType: "image/jpeg",
		Size: 1, StorageKind: "local", SecurityStatus: files.SecurityClean, ScanStatus: files.SecurityClean,
	}
	db.Create(f)
	access := &files.ObjectAccessService{DB: db, Cfg: testCfg()}
	c := ginWithTenant(tenantA, uuid.New())
	_, err := access.LoadForDownload(c.Request.Context(), tenantA, f.ID)
	if err == nil {
		t.Fatal("assetId cross-tenant must fail even if objectKey guessed")
	}
}

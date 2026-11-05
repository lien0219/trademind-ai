package operationlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newOperationLogTestDB(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&OperationLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHashChainConcurrentAppendSameScope(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- svc.WriteBackground(context.Background(), WriteOpts{
				TenantID: 7,
				Action:   "login",
				Resource: "auth",
				Status:   "failed",
				Message:  fmt.Sprintf("wrong password %d", i),
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	var rows []OperationLog
	if err := db.Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != n {
		t.Fatalf("rows = %d, want %d", len(rows), n)
	}

	prev := ""
	predecessorUse := map[string]int{}
	for i := range rows {
		if rows[i].EntryHash == "" {
			t.Fatalf("row %d missing entry hash", i)
		}
		if rows[i].PrevHash != prev {
			t.Fatalf("row %d prev hash mismatch: got %q want %q", i, rows[i].PrevHash, prev)
		}
		predecessorUse[rows[i].PrevHash]++
		prev = rows[i].EntryHash
	}
	if predecessorUse[""] != 1 {
		t.Fatalf("genesis predecessor count = %d, want 1", predecessorUse[""])
	}
	for predecessor, count := range predecessorUse {
		if predecessor != "" && count != 1 {
			t.Fatalf("predecessor %q used %d times, want 1", predecessor, count)
		}
	}
	count, mismatchAt, err := svc.VerifyChain(context.Background(), 7, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("verify chain mismatch at %v: %v", mismatchAt, err)
	}
	if count != n {
		t.Fatalf("verify count = %d, want %d", count, n)
	}
}

func TestWriteBootstrapAuditTxParticipatesInCallerTransaction(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	rollbackErr := errors.New("force bootstrap rollback")
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := svc.WriteBootstrapAuditTx(ctx, tx, 0, "admin.bootstrap.create", "admin_user", uuid.NewString(), "role=admin status=active"); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v, want %v", err, rollbackErr)
	}
	var count int64
	if err := db.Model(&OperationLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("persisted %d audit rows from a rolled back transaction", count)
	}

	resourceID := uuid.NewString()
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return svc.WriteBootstrapAuditTx(ctx, tx, 0, "admin.bootstrap.create", "admin_user", resourceID, "role=admin status=active")
	}); err != nil {
		t.Fatalf("write bootstrap audit: %v", err)
	}
	var row OperationLog
	if err := db.Where("resource_id = ?", resourceID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "success" || row.Method != "INTERNAL" || row.EntryHash == "" {
		t.Fatalf("incomplete bootstrap audit row: %+v", row)
	}
	verifiedCount, _, err := svc.VerifyChain(ctx, 0, row.CreatedAt.Add(-time.Hour), row.CreatedAt.Add(time.Hour))
	if err != nil || verifiedCount != 1 {
		t.Fatalf("bootstrap audit chain verification count=%d err=%v", verifiedCount, err)
	}
}

func TestWriteTxContextParticipatesInCallerTransaction(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()
	rollbackErr := errors.New("force internal audit rollback")
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := svc.WriteTxContext(ctx, tx, WriteOpts{
			TenantID: 7, AdminRole: "worker", Action: "shop.oauth.refresh", Resource: "shop",
			ResourceID: uuid.NewString(), Status: "success", Message: "token refreshed",
		}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v, want %v", err, rollbackErr)
	}
	var count int64
	if err := db.Model(&OperationLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("persisted %d internal audit rows from rollback", count)
	}
}

func TestPreviousHashLookupFailureDoesNotWriteEmptyHash(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	forced := errors.New("forced previous hash lookup failure")
	if err := db.Callback().Query().Before("gorm:query").Register("operationlog:test_query_fail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "operation_logs" {
			tx.AddError(forced)
		}
	}); err != nil {
		t.Fatal(err)
	}
	err := svc.WriteBackground(context.Background(), WriteOpts{TenantID: 9, Action: "login", Resource: "auth", Status: "failed"})
	if !errors.Is(err, forced) {
		t.Fatalf("err = %v, want forced lookup failure", err)
	}
	_ = db.Callback().Query().Remove("operationlog:test_query_fail")

	var count int64
	if err := db.Model(&OperationLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rows after lookup failure = %d, want 0", count)
	}
}

func TestOperationLogInsertFailureRollsBackHashChain(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	forced := errors.New("forced operation log insert failure")
	if err := db.Callback().Create().Before("gorm:create").Register("operationlog:test_create_fail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "operation_logs" {
			tx.AddError(forced)
		}
	}); err != nil {
		t.Fatal(err)
	}
	err := svc.WriteBackground(context.Background(), WriteOpts{TenantID: 11, Action: "login", Resource: "auth", Status: "failed"})
	if !errors.Is(err, forced) {
		t.Fatalf("err = %v, want forced insert failure", err)
	}
	_ = db.Callback().Create().Remove("operationlog:test_create_fail")

	var count int64
	if err := db.Model(&OperationLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rows after insert failure = %d, want 0", count)
	}
}

func TestVerifyChainResetsPredecessorAtDatePartitionBoundary(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	firstAt := time.Date(2026, 1, 1, 23, 59, 0, 0, time.UTC)
	secondAt := time.Date(2026, 1, 2, 0, 1, 0, 0, time.UTC)
	rows := []OperationLog{
		{ID: uuid.New(), TenantID: 17, Action: "user.update", Resource: "admin_user", Status: "success", CreatedAt: firstAt, ChainPartition: chainPartition(17, firstAt)},
		{ID: uuid.New(), TenantID: 17, Action: "role.update", Resource: "admin_role", Status: "success", CreatedAt: secondAt, ChainPartition: chainPartition(17, secondAt)},
	}
	for i := range rows {
		rows[i].PrevHash = ""
		rows[i].EntryHash = computeEntryHash("", &rows[i])
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create audit row: %v", err)
		}
	}

	count, mismatchAt, err := svc.VerifyChain(context.Background(), 17, firstAt.Add(-time.Minute), secondAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("verify partition genesis rows at %v: %v", mismatchAt, err)
	}
	if count != 2 {
		t.Fatalf("verify count = %d, want 2", count)
	}
}

func TestVerifyChainAnchorsPartialDayRangeToStoredPredecessor(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	firstAt := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	secondAt := firstAt.Add(time.Hour)
	first := OperationLog{ID: uuid.New(), TenantID: 17, Action: "user.update", Resource: "admin_user", Status: "success", CreatedAt: firstAt, ChainPartition: chainPartition(17, firstAt)}
	first.EntryHash = computeEntryHash("", &first)
	second := OperationLog{ID: uuid.New(), TenantID: 17, Action: "role.update", Resource: "admin_role", Status: "success", CreatedAt: secondAt, ChainPartition: chainPartition(17, secondAt), PrevHash: first.EntryHash}
	second.EntryHash = computeEntryHash(first.EntryHash, &second)
	for _, row := range []OperationLog{first, second} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	from := firstAt.Add(30 * time.Minute)
	count, _, err := svc.VerifyChain(context.Background(), 17, from, secondAt.Add(time.Minute))
	if err != nil || count != 1 {
		t.Fatalf("verify partial-day range = (%d, %v), want (1, nil)", count, err)
	}
	if err := db.Model(&first).Update("message", "tampered predecessor").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.VerifyChain(context.Background(), 17, from, secondAt.Add(time.Minute)); err == nil {
		t.Fatal("partial-day verification accepted a tampered predecessor")
	}
}

func TestHashChainAppendUsesNewPartitionPredecessorAfterTimestampCrossesMidnight(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	firstAt := time.Date(2026, 1, 1, 23, 59, 59, 999999000, time.UTC)
	secondAt := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	first := OperationLog{ID: uuid.New(), TenantID: 17, Action: "first", Resource: "admin_user", Status: "success", CreatedAt: firstAt, ChainPartition: chainPartition(17, firstAt)}
	first.EntryHash = computeEntryHash("", &first)
	second := OperationLog{ID: uuid.New(), TenantID: 17, Action: "second", Resource: "admin_role", Status: "success", CreatedAt: secondAt, ChainPartition: chainPartition(17, secondAt)}
	second.EntryHash = computeEntryHash("", &second)
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first partition row: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second partition row: %v", err)
	}

	appended := OperationLog{ID: uuid.New(), TenantID: 17, Action: "third", Resource: "admin_user", Status: "success", CreatedAt: firstAt}
	var releaseChainLocks func()
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		releaseChainLocks, err = svc.appendHashChain(tx, &appended)
		if err != nil {
			return err
		}
		return tx.Create(&appended).Error
	}); err != nil {
		t.Fatalf("append row across UTC midnight: %v", err)
	}
	if releaseChainLocks != nil {
		releaseChainLocks()
	}

	if appended.ChainPartition != chainPartition(17, secondAt) {
		t.Fatalf("partition = %q, want %q", appended.ChainPartition, chainPartition(17, secondAt))
	}
	if appended.PrevHash != second.EntryHash {
		t.Fatalf("previous hash = %q, want latest hash in new partition %q", appended.PrevHash, second.EntryHash)
	}
	if !appended.CreatedAt.After(second.CreatedAt) {
		t.Fatalf("created at = %s, want after new partition predecessor %s", appended.CreatedAt, second.CreatedAt)
	}
	if _, _, err := svc.VerifyChain(context.Background(), 17, firstAt.Add(-time.Minute), secondAt.Add(time.Minute)); err != nil {
		t.Fatalf("verify chain after cross-partition append: %v", err)
	}
}

func TestWriteTxRetainsCompletePermissionChangeEvidence(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("PUT", "/api/v1/admin/users/test/store-permissions", nil)
	grants := make([]map[string]string, 32)
	for i := range grants {
		grants[i] = map[string]string{"storeId": uuid.NewString(), "permissionScope": "operate"}
	}
	evidence, err := json.Marshal(map[string]any{"before": grants, "after": grants})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) <= 2000 {
		t.Fatal("fixture must exceed the legacy audit truncation limit")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return svc.WriteTx(c, tx, WriteOpts{TenantID: 7, AdminRole: "admin", Action: "user.store_permissions.set", Resource: "admin_user", Status: "success", Message: string(evidence)})
	}); err != nil {
		t.Fatal(err)
	}
	var row OperationLog
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Message != string(evidence) || !json.Valid([]byte(row.Message)) {
		t.Fatalf("permission change evidence was truncated: got %d bytes, want %d", len(row.Message), len(evidence))
	}
	if _, _, err := svc.VerifyChain(context.Background(), 7, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyChainDetectsActorAttributionTamper(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	actorID := uuid.New()
	if err := svc.WriteBackground(context.Background(), WriteOpts{
		TenantID: 7, AdminUserID: &actorID, AdminRole: "admin", Username: "operator@example.test",
		Action: "user.role.update", Resource: "admin_user", ResourceID: actorID.String(), Status: "success",
	}); err != nil {
		t.Fatal(err)
	}
	var row OperationLog
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	requireAdminRole := "readonly"
	if err := db.Model(&row).Update("admin_role", requireAdminRole).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.VerifyChain(context.Background(), 7, row.CreatedAt.Add(-time.Hour), row.CreatedAt.Add(time.Hour)); err == nil {
		t.Fatal("actor attribution tamper was not detected")
	}
}

func TestVerifyChainRejectsMissingEntryHash(t *testing.T) {
	db := newOperationLogTestDB(t)
	svc := &Service{DB: db}
	row := OperationLog{
		ID: uuid.New(), TenantID: 8, Action: "user.disable", Resource: "admin_user", Status: "success",
		CreatedAt: time.Now().UTC(), ChainPartition: chainPartition(8, time.Now().UTC()),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.VerifyChain(context.Background(), 8, row.CreatedAt.Add(-time.Hour), row.CreatedAt.Add(time.Hour)); err == nil {
		t.Fatal("missing entry hash was accepted")
	}
}

func BenchmarkOperationLogHashChainAppend(b *testing.B) {
	db := newOperationLogTestDB(b)
	svc := &Service{DB: db}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := svc.WriteBackground(context.Background(), WriteOpts{
			TenantID: 17,
			Action:   "login",
			Resource: "auth",
			Status:   "failed",
		}); err != nil {
			b.Fatal(err)
		}
	}
}

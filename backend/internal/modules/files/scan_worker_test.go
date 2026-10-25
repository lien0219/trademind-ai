package files

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/filescanner"
	"github.com/trademind-ai/trademind/backend/internal/pkg/taskqueue"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

type cleanScanWorkerScanner struct{}

func (cleanScanWorkerScanner) Name() string { return "test-clean" }

func (cleanScanWorkerScanner) Scan(context.Context, filescanner.ScanInput) (filescanner.ScanResult, error) {
	return filescanner.ScanResult{Status: filescanner.ResultClean}, nil
}

func newScanWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:scan_worker_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&FileRecord{}, &settings.Setting{}, &admin.Tenant{}))
	for id := int64(40); id <= 50; id++ {
		require.NoError(t, db.Create(&admin.Tenant{ID: id, Code: "scan-worker-tenant-" + strconv.FormatInt(id, 10), Name: "scan-worker-tenant", Status: admin.TenantActive}).Error)
	}
	return db
}

func TestProcessScanPayloadResumesStaleScanningFile(t *testing.T) {
	db := newScanWorkerTestDB(t)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "asset.bin"), []byte("test payload"), 0o600))
	for _, row := range []settings.Setting{
		{TenantID: 42, GroupKey: "storage", ItemKey: "kind", ItemValue: "local"},
		{TenantID: 42, GroupKey: "storage", ItemKey: "local_root", ItemValue: root},
		{TenantID: 42, GroupKey: "storage", ItemKey: "public_base", ItemValue: "/static"},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	row := FileRecord{
		ID: uuid.New(), TenantID: 42, OriginalName: "asset.bin", ObjectKey: "asset.bin",
		PublicURL: "/static/asset.bin", ContentType: "application/octet-stream", Size: 12,
		StorageKind: "local", SecurityStatus: SecurityScanning, ScanStatus: SecurityScanning,
	}
	require.NoError(t, db.Create(&row).Error)
	payload, err := json.Marshal(ScanQueueMessage{TenantID: row.TenantID, AssetID: row.ID.String()})
	require.NoError(t, err)

	svc := &Service{DB: db, Settings: &settings.Service{DB: db}}
	require.NoError(t, svc.processScanPayload(context.Background(), slog.Default(), cleanScanWorkerScanner{}, string(payload)))

	var stored FileRecord
	require.NoError(t, db.First(&stored, "id = ?", row.ID).Error)
	require.Equal(t, SecurityClean, stored.SecurityStatus)
	require.Equal(t, SecurityClean, stored.ScanStatus)
}

func TestProcessScanPayloadReturnsErrorWhenFinalStatusWriteFails(t *testing.T) {
	db := newScanWorkerTestDB(t)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "asset.bin"), []byte("test payload"), 0o600))
	for _, row := range []settings.Setting{
		{TenantID: 43, GroupKey: "storage", ItemKey: "kind", ItemValue: "local"},
		{TenantID: 43, GroupKey: "storage", ItemKey: "local_root", ItemValue: root},
		{TenantID: 43, GroupKey: "storage", ItemKey: "public_base", ItemValue: "/static"},
	} {
		require.NoError(t, db.Create(&row).Error)
	}
	row := FileRecord{
		ID: uuid.New(), TenantID: 43, OriginalName: "asset.bin", ObjectKey: "asset.bin",
		PublicURL: "/static/asset.bin", ContentType: "application/octet-stream", Size: 12,
		StorageKind: "local", SecurityStatus: SecurityPendingScan, ScanStatus: SecurityPendingScan,
	}
	require.NoError(t, db.Create(&row).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_clean_scan_status BEFORE UPDATE OF security_status ON files WHEN NEW.security_status = 'clean' BEGIN SELECT RAISE(ABORT, 'test final status failure'); END`).Error)
	payload, err := json.Marshal(ScanQueueMessage{TenantID: row.TenantID, AssetID: row.ID.String()})
	require.NoError(t, err)

	svc := &Service{DB: db, Settings: &settings.Service{DB: db}}
	err = svc.processScanPayload(context.Background(), slog.Default(), cleanScanWorkerScanner{}, string(payload))
	require.Error(t, err)

	var stored FileRecord
	require.NoError(t, db.First(&stored, "id = ?", row.ID).Error)
	require.Equal(t, SecurityScanning, stored.SecurityStatus)
}

func TestProcessScanPayloadDoesNotResetFreshScanClaim(t *testing.T) {
	db := newScanWorkerTestDB(t)
	now := time.Now().UTC()
	row := FileRecord{
		ID:             uuid.New(),
		TenantID:       45,
		OriginalName:   "asset.bin",
		ObjectKey:      "asset-45.bin",
		PublicURL:      "/static/asset-45.bin",
		SecurityStatus: SecurityScanning,
		ScanStatus:     SecurityScanning,
		ScanClaimID:    "live-claim",
		ScanClaimedAt:  &now,
	}
	require.NoError(t, db.Create(&row).Error)
	payload, err := json.Marshal(ScanQueueMessage{TenantID: row.TenantID, AssetID: row.ID.String()})
	require.NoError(t, err)

	svc := &Service{DB: db, Settings: &settings.Service{DB: db}}
	err = svc.processScanPayload(context.Background(), slog.Default(), cleanScanWorkerScanner{}, string(payload))
	require.ErrorIs(t, err, errScanAlreadyClaimed)

	var stored FileRecord
	require.NoError(t, db.First(&stored, "id = ?", row.ID).Error)
	require.Equal(t, SecurityScanning, stored.SecurityStatus)
	require.Equal(t, "live-claim", stored.ScanClaimID)
}

func TestClaimScanReplacesOnlyStaleClaim(t *testing.T) {
	db := newScanWorkerTestDB(t)
	old := time.Now().UTC().Add(-scanClaimStaleAfter - time.Second)
	row := FileRecord{
		ID:             uuid.New(),
		TenantID:       46,
		OriginalName:   "asset.bin",
		ObjectKey:      "asset-46.bin",
		PublicURL:      "/static/asset-46.bin",
		SecurityStatus: SecurityScanning,
		ScanStatus:     SecurityScanning,
		ScanClaimID:    "stale-claim",
		ScanClaimedAt:  &old,
	}
	require.NoError(t, db.Create(&row).Error)

	claimed, err := claimScan(context.Background(), db, row.ID, row.TenantID, "new-claim", time.Now().UTC())
	require.NoError(t, err)
	require.True(t, claimed)

	var stored FileRecord
	require.NoError(t, db.First(&stored, "id = ?", row.ID).Error)
	require.Equal(t, SecurityScanning, stored.SecurityStatus)
	require.Equal(t, "new-claim", stored.ScanClaimID)
}

func TestMarkScanFailedRequiresTenantScopedRow(t *testing.T) {
	db := newScanWorkerTestDB(t)
	row := FileRecord{ID: uuid.New(), TenantID: 44, OriginalName: "asset.bin", ObjectKey: "asset-44.bin", PublicURL: "/static/asset-44.bin", SecurityStatus: SecurityPendingScan, ScanStatus: SecurityPendingScan}
	require.NoError(t, db.Create(&row).Error)
	require.Error(t, markScanFailed(context.Background(), db, row.ID, 45))
	var stored FileRecord
	require.NoError(t, db.First(&stored, "id = ?", row.ID).Error)
	require.Equal(t, SecurityPendingScan, stored.SecurityStatus)
}

func TestStartScanWorkerRequeuesFreshClaimInsteadOfAcknowledging(t *testing.T) {
	cfg, ok, err := safeenv.TestRedisURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_REDIS_URL is required for scan claim recovery")
	}
	options, err := goredis.ParseURL(cfg.URL)
	require.NoError(t, err)
	client := goredis.NewClient(options)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	require.NoError(t, client.Ping(ctx).Err())
	require.NoError(t, client.Del(ctx, fileScanQueueName, taskqueue.ConsumerHeartbeatKey(fileScanQueueName)).Err())

	db := newScanWorkerTestDB(t)
	now := time.Now().UTC()
	row := FileRecord{
		ID: uuid.New(), TenantID: 47, OriginalName: "asset.bin", ObjectKey: "asset-47.bin",
		PublicURL: "/static/asset-47.bin", SecurityStatus: SecurityScanning, ScanStatus: SecurityScanning,
		ScanClaimID: "crashed-claim", ScanClaimedAt: &now,
	}
	require.NoError(t, db.Create(&row).Error)
	payload, err := json.Marshal(ScanQueueMessage{TenantID: row.TenantID, AssetID: row.ID.String()})
	require.NoError(t, err)
	require.NoError(t, client.LPush(ctx, fileScanQueueName, string(payload)).Err())

	var wg sync.WaitGroup
	StartScanWorker(ctx, &wg, slog.Default(), &Service{DB: db, Redis: &rdb.Client{Client: client}}, nil, nil)
	require.Eventually(t, func() bool {
		return client.HLen(ctx, taskqueue.ConsumerHeartbeatKey(fileScanQueueName)).Val() > 0
	}, time.Second, 20*time.Millisecond, "the worker must register before the recovery assertion")
	time.Sleep(1500 * time.Millisecond)
	processingKeys, err := client.Keys(ctx, taskqueue.ProcessingQueue(fileScanQueueName)+"*").Result()
	require.NoError(t, err)
	outstanding := client.LLen(ctx, fileScanQueueName).Val()
	for _, key := range processingKeys {
		outstanding += client.LLen(ctx, key).Val()
	}
	require.Equal(t, int64(1), outstanding, "a fresh durable claim must remain retryable after queue recovery")
	cancel()
	wg.Wait()
}

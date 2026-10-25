//go:build integration || inventorypostgres

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/taskqueue"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

func TestOrderSyncWorkerRetriesTransientClaimFailure(t *testing.T) {
	requireSyncWorkerIntegration(t)
	f := newTenantPGFixture(t)
	require.NoError(t, f.db.AutoMigrate(&ordersync.OrderSyncTask{}))
	redisClient, client := newSyncWorkerRedis(t)

	owner := f.user(t, "order-sync-retry")
	tenant := f.tenant(t, owner, "order-sync-retry")
	store := shop.Shop{TenantID: tenant.ID, Platform: "mock", ShopName: "order-sync-retry-shop", Status: shop.StatusActive}
	require.NoError(t, f.db.Create(&store).Error)
	task := ordersync.OrderSyncTask{TenantID: tenant.ID, ShopID: store.ID, Platform: "mock", TaskType: ordersync.TaskTypeOrderSync, Status: ordersync.StatusPending, Mode: ordersync.ModeIncremental}
	require.NoError(t, f.db.Create(&task).Error)
	installSyncWorkerTrigger(t, f.db, "order_sync_tasks", "pending", "running")

	queue := "test:sync-worker:order-claim:" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	ordersync.StartWorker(ctx, &wg, nil, &ordersync.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
	t.Cleanup(func() { stopSyncWorker(t, cancel, &wg, client, queue) })
	require.NoError(t, client.LPush(ctx, queue, fmt.Sprintf(`{"taskId":%q}`, task.ID.String())).Err())

	var state struct {
		Status string
	}
	require.Eventually(t, func() bool {
		return f.db.Model(&ordersync.OrderSyncTask{}).Select("status").Where("id = ?", task.ID).Take(&state).Error == nil && state.Status == ordersync.StatusFailed
	}, 5*time.Second, 25*time.Millisecond)

	var attempts int64
	require.NoError(t, f.db.Raw("SELECT last_value FROM sync_worker_claim_failure_seq").Scan(&attempts).Error)
	require.GreaterOrEqual(t, attempts, int64(2), "the retry must reach PostgreSQL after the first claim update failure")
	require.Eventually(t, func() bool { return client.LLen(ctx, queue).Val() == 0 }, time.Second, 20*time.Millisecond)

}

func TestCustomerSyncWorkerRetriesTransientRejectionFailure(t *testing.T) {
	requireSyncWorkerIntegration(t)
	f := newTenantPGFixture(t)
	require.NoError(t, f.db.AutoMigrate(&customersync.CustomerMessageSyncTask{}))
	redisClient, client := newSyncWorkerRedis(t)

	owner := f.user(t, "customer-sync-retry")
	tenant := f.tenant(t, owner, "customer-sync-retry")
	store := shop.Shop{TenantID: tenant.ID, Platform: "mock", ShopName: "customer-sync-retry-shop", Status: shop.StatusActive}
	require.NoError(t, f.db.Create(&store).Error)
	require.NoError(t, f.db.Model(&admin.Tenant{}).Where("id = ?", tenant.ID).Update("status", admin.TenantSuspended).Error)
	task := customersync.CustomerMessageSyncTask{TenantID: tenant.ID, ShopID: store.ID, Platform: "mock", TaskType: customersync.TaskTypeCustomerMessageSync, Status: customersync.StatusPending, Mode: customersync.ModeIncremental}
	require.NoError(t, f.db.Create(&task).Error)
	installSyncWorkerTrigger(t, f.db, "customer_message_sync_tasks", "pending", "failed")

	queue := "test:sync-worker:customer-rejection:" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	customersync.StartWorker(ctx, &wg, nil, &customersync.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
	t.Cleanup(func() { stopSyncWorker(t, cancel, &wg, client, queue) })
	require.NoError(t, client.LPush(ctx, queue, fmt.Sprintf(`{"taskId":%q}`, task.ID.String())).Err())

	var state struct {
		Status string
	}
	require.Eventually(t, func() bool {
		return f.db.Model(&customersync.CustomerMessageSyncTask{}).Select("status").Where("id = ?", task.ID).Take(&state).Error == nil && state.Status == customersync.StatusFailed
	}, 5*time.Second, 25*time.Millisecond)

	var attempts int64
	require.NoError(t, f.db.Raw("SELECT last_value FROM sync_worker_rejection_failure_seq").Scan(&attempts).Error)
	require.GreaterOrEqual(t, attempts, int64(2), "the rejection write must be retried after its first PostgreSQL failure")
	require.Eventually(t, func() bool { return client.LLen(ctx, queue).Val() == 0 }, time.Second, 20*time.Millisecond)

}

func TestOrderSyncWorkerRetainsNonTerminalClaimMiss(t *testing.T) {
	requireSyncWorkerIntegration(t)
	f := newTenantPGFixture(t)
	require.NoError(t, f.db.AutoMigrate(&ordersync.OrderSyncTask{}))
	redisClient, client := newSyncWorkerRedis(t)

	owner := f.user(t, "order-sync-claim-miss")
	tenant := f.tenant(t, owner, "order-sync-claim-miss")
	store := shop.Shop{TenantID: tenant.ID, Platform: "mock", ShopName: "order-sync-claim-miss-shop", Status: shop.StatusActive}
	require.NoError(t, f.db.Create(&store).Error)
	lockedBy := "another-order-worker"
	lockedUntil := time.Now().UTC().Add(time.Minute)
	executionID := uuid.NewString()
	task := ordersync.OrderSyncTask{TenantID: tenant.ID, ShopID: store.ID, Platform: "mock", TaskType: ordersync.TaskTypeOrderSync, Status: ordersync.StatusRunning, Mode: ordersync.ModeIncremental, LockedBy: &lockedBy, LockedUntil: &lockedUntil, ExecutionID: &executionID, LockVersion: 1}
	require.NoError(t, f.db.Create(&task).Error)

	queue := "test:sync-worker:order-claim-miss:" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	ordersync.StartWorker(ctx, &wg, nil, &ordersync.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
	t.Cleanup(func() { stopSyncWorker(t, cancel, &wg, client, queue) })
	payload := fmt.Sprintf(`{"taskId":%q}`, task.ID.String())
	require.NoError(t, client.LPush(ctx, queue, payload).Err())

	require.Eventually(t, func() bool { return processingQueueLength(ctx, client, queue) == 1 }, 3*time.Second, 20*time.Millisecond, "a non-terminal claim miss must retain its reservation")
	require.Zero(t, client.LLen(ctx, queue).Val(), "claim miss must not hot-loop the ready queue")
}

func TestCustomerSyncWorkerRetainsNonTerminalClaimMiss(t *testing.T) {
	requireSyncWorkerIntegration(t)
	f := newTenantPGFixture(t)
	require.NoError(t, f.db.AutoMigrate(&customersync.CustomerMessageSyncTask{}))
	redisClient, client := newSyncWorkerRedis(t)

	owner := f.user(t, "customer-sync-claim-miss")
	tenant := f.tenant(t, owner, "customer-sync-claim-miss")
	store := shop.Shop{TenantID: tenant.ID, Platform: "mock", ShopName: "customer-sync-claim-miss-shop", Status: shop.StatusActive}
	require.NoError(t, f.db.Create(&store).Error)
	lockedBy := "another-customer-worker"
	lockedUntil := time.Now().UTC().Add(time.Minute)
	executionID := uuid.NewString()
	task := customersync.CustomerMessageSyncTask{TenantID: tenant.ID, ShopID: store.ID, Platform: "mock", TaskType: customersync.TaskTypeCustomerMessageSync, Status: customersync.StatusRunning, Mode: customersync.ModeIncremental, LockedBy: &lockedBy, LockedUntil: &lockedUntil, ExecutionID: &executionID, LockVersion: 1}
	require.NoError(t, f.db.Create(&task).Error)

	queue := "test:sync-worker:customer-claim-miss:" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	customersync.StartWorker(ctx, &wg, nil, &customersync.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
	t.Cleanup(func() { stopSyncWorker(t, cancel, &wg, client, queue) })
	payload := fmt.Sprintf(`{"taskId":%q}`, task.ID.String())
	require.NoError(t, client.LPush(ctx, queue, payload).Err())

	require.Eventually(t, func() bool { return processingQueueLength(ctx, client, queue) == 1 }, 3*time.Second, 20*time.Millisecond, "a non-terminal claim miss must retain its reservation")
	require.Zero(t, client.LLen(ctx, queue).Val(), "claim miss must not hot-loop the ready queue")
}

func newSyncWorkerRedis(t *testing.T) (*rdb.Client, *goredis.Client) {
	t.Helper()
	redisConfig, ok, err := safeenv.TestRedisURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_REDIS_URL is required for sync worker integration")
	}
	options, err := goredis.ParseURL(redisConfig.URL)
	require.NoError(t, err)
	client := goredis.NewClient(options)
	require.NoError(t, client.Ping(context.Background()).Err())
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return &rdb.Client{Client: client}, client
}

func requireSyncWorkerIntegration(t *testing.T) {
	t.Helper()
	_, dbOK, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !dbOK {
		t.Skip("TEST_DATABASE_URL is required for sync worker integration")
	}
	_, redisOK, err := safeenv.TestRedisURLFromEnv()
	require.NoError(t, err)
	if !redisOK {
		t.Skip("TEST_REDIS_URL is required for sync worker integration")
	}
}

func installSyncWorkerTrigger(t *testing.T, db *gorm.DB, table, oldStatus, newStatus string) {
	t.Helper()
	sequence := "sync_worker_" + map[string]string{"order_sync_tasks": "claim", "customer_message_sync_tasks": "rejection"}[table] + "_failure_seq"
	function := sequence + "_fn"
	trigger := sequence + "_trigger"
	require.NoError(t, db.Exec(fmt.Sprintf(`CREATE SEQUENCE %s START 1`, sequence)).Error)
	require.NoError(t, db.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = '%s' AND NEW.status = '%s' AND nextval('%s') = 1 THEN
    RAISE EXCEPTION 'forced sync worker persistence failure';
  END IF;
  RETURN NEW;
END
$$`, function, oldStatus, newStatus, sequence)).Error)
	require.NoError(t, db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION %s()`, trigger, table, function)).Error)
	t.Cleanup(func() {
		_ = db.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON %s`, trigger, table)).Error
		_ = db.Exec(fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, function)).Error
		_ = db.Exec(fmt.Sprintf(`DROP SEQUENCE IF EXISTS %s`, sequence)).Error
	})
}

func stopSyncWorker(t *testing.T, cancel context.CancelFunc, wg *sync.WaitGroup, client *goredis.Client, queue string) {
	t.Helper()
	cancel()
	_ = client.LPush(context.Background(), queue, `{}`).Err()
	wg.Wait()
	_ = client.Del(context.Background(), queue, taskqueue.ConsumerHeartbeatKey(queue)).Err()
}

func processingQueueLength(ctx context.Context, client *goredis.Client, queue string) int64 {
	keys, err := client.Keys(ctx, taskqueue.ProcessingQueue(queue)+"*").Result()
	if err != nil {
		return 0
	}
	var total int64
	for _, key := range keys {
		total += client.LLen(ctx, key).Val()
	}
	return total
}

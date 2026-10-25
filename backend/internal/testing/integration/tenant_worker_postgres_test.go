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
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

func TestTenantSuspensionLeavesDequeuedWorkRetryable(t *testing.T) {
	f := newTenantPGFixture(t)
	require.NoError(t, f.db.AutoMigrate(&ordersync.OrderSyncTask{}, &customersync.CustomerMessageSyncTask{}, &inventory.InventorySyncTask{}, &productpublish.ProductPublishTask{}, &imagetask.ImageTask{}))
	redisConfig, ok, err := safeenv.TestRedisURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_REDIS_URL is required for worker lifecycle integration")
	}
	options, err := goredis.ParseURL(redisConfig.URL)
	require.NoError(t, err)
	client := goredis.NewClient(options)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, client.Ping(context.Background()).Err())
	redisClient := &rdb.Client{Client: client}
	owner := f.user(t, "worker-owner")
	tenant := f.tenant(t, owner, "worker-suspended")
	store := shop.Shop{TenantID: tenant.ID, Platform: "shopee", ShopName: "worker-test-shop", Status: "active"}
	require.NoError(t, f.db.Create(&store).Error)
	require.NoError(t, f.db.Model(&admin.Tenant{}).Where("id = ?", tenant.ID).Update("status", admin.TenantSuspended).Error)
	cases := []struct {
		name  string
		model any
		start func(context.Context, *sync.WaitGroup, string)
	}{
		{"orders", &ordersync.OrderSyncTask{TenantID: tenant.ID, ShopID: store.ID, Status: "pending"}, func(ctx context.Context, wg *sync.WaitGroup, queue string) {
			ordersync.StartWorker(ctx, wg, nil, &ordersync.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
		}},
		{"customers", &customersync.CustomerMessageSyncTask{TenantID: tenant.ID, ShopID: store.ID, Status: "pending"}, func(ctx context.Context, wg *sync.WaitGroup, queue string) {
			customersync.StartWorker(ctx, wg, nil, &customersync.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
		}},
		{"inventory", &inventory.InventorySyncTask{TenantID: tenant.ID, ShopID: store.ID, Status: "pending", ProductID: uuid.New()}, func(ctx context.Context, wg *sync.WaitGroup, queue string) {
			inventory.StartWorker(ctx, wg, nil, &inventory.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
		}},
		{"publish", &productpublish.ProductPublishTask{TenantID: tenant.ID, ShopID: store.ID, Status: "pending", ProductID: uuid.New()}, func(ctx context.Context, wg *sync.WaitGroup, queue string) {
			productpublish.StartWorker(ctx, wg, nil, &productpublish.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
		}},
		{"images", &imagetask.ImageTask{TenantID: tenant.ID, Status: "retrying"}, func(ctx context.Context, wg *sync.WaitGroup, queue string) {
			imagetask.StartWorker(ctx, wg, nil, &imagetask.Service{DB: f.db, Redis: redisClient}, queue, 1, nil)
		}},
	}
	for _, entry := range cases {
		t.Run(entry.name, func(t *testing.T) {
			require.NoError(t, f.db.Create(entry.model).Error)
			var task struct{ ID uuid.UUID }
			require.NoError(t, f.db.Model(entry.model).Select("id").Where("tenant_id = ?", tenant.ID).Take(&task).Error)
			queue := "test:trademind:tenant-worker:" + uuid.NewString()
			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			defer func() {
				cancel()
				// Wake BRPOP so shutdown does not depend on its network timeout.
				_ = client.LPush(context.Background(), queue, `{}`).Err()
				wg.Wait()
				_ = client.Del(context.Background(), queue).Err()
			}()
			entry.start(ctx, &wg, queue)
			require.NoError(t, client.LPush(ctx, queue, fmt.Sprintf(`{"taskId":%q}`, task.ID.String())).Err())
			var state struct {
				Status       string
				ErrorMessage string
				FinishedAt   *time.Time
			}
			require.Eventually(t, func() bool {
				return f.db.Model(entry.model).Select("status, error_message, finished_at").Where("id = ?", task.ID).Take(&state).Error == nil && state.Status == "failed"
			}, 3*time.Second, 20*time.Millisecond, "dequeued work must reach a visible, retryable terminal state")
			require.NotNil(t, state.FinishedAt)
			require.NotEmpty(t, state.ErrorMessage)
			require.Zero(t, client.LLen(ctx, queue).Val(), "lifecycle denial must not create a hot requeue loop")
		})
	}
}

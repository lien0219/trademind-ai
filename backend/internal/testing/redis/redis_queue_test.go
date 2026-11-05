package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/taskqueue"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

func openTestRedis(t *testing.T) (*goredis.Client, context.Context) {
	t.Helper()
	cfg, ok, err := safeenv.TestRedisURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_REDIS_URL is not set; skipping Redis queue integration test")
	}
	options, err := goredis.ParseURL(cfg.URL)
	require.NoError(t, err)
	client := goredis.NewClient(options)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, client.Ping(ctx).Err())
	return client, ctx
}

func TestRedisListQueueRoundTripInIsolatedDB(t *testing.T) {
	client, ctx := openTestRedis(t)
	key := "test:trademind:queue:roundtrip"
	require.NoError(t, client.Del(ctx, key).Err())
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })

	require.NoError(t, client.LPush(ctx, key, `{"taskId":"test-task-1"}`).Err())
	item, err := client.BRPop(ctx, time.Second, key).Result()
	require.NoError(t, err)
	require.Equal(t, []string{key, `{"taskId":"test-task-1"}`}, item)
}

func TestRedisReliableListReservationAckAndRequeue(t *testing.T) {
	client, ctx := openTestRedis(t)
	ready := "test:trademind:queue:reliable"
	consumerID := "consumer-roundtrip"
	processing := taskqueue.ConsumerProcessingQueue(ready, consumerID)
	payload := `{"taskId":"test-task-reliable"}`
	require.NoError(t, client.Del(ctx, ready, processing).Err())
	t.Cleanup(func() { _ = client.Del(context.Background(), ready, processing).Err() })

	require.NoError(t, client.LPush(ctx, ready, payload).Err())
	reserved, err := taskqueue.Reserve(ctx, client, ready, consumerID, time.Second)
	require.NoError(t, err)
	require.Equal(t, payload, reserved)
	require.Equal(t, int64(0), client.LLen(ctx, ready).Val())
	require.Equal(t, int64(1), client.LLen(ctx, processing).Val())

	require.NoError(t, taskqueue.Requeue(ctx, client, ready, consumerID, reserved))
	require.Equal(t, int64(1), client.LLen(ctx, ready).Val())
	require.Equal(t, int64(0), client.LLen(ctx, processing).Val())

	reserved, err = taskqueue.Reserve(ctx, client, ready, consumerID, time.Second)
	require.NoError(t, err)
	require.NoError(t, taskqueue.Ack(ctx, client, ready, consumerID, reserved))
	require.Equal(t, int64(0), client.LLen(ctx, ready).Val())
	require.Equal(t, int64(0), client.LLen(ctx, processing).Val())
}

func TestRedisRecoverySkipsWhenAnotherConsumerHeartbeatIsLive(t *testing.T) {
	client, ctx := openTestRedis(t)
	ready := "test:trademind:queue:recovery-guard"
	processing := taskqueue.ProcessingQueue(ready)
	require.NoError(t, client.Del(ctx, ready, processing, taskqueue.ConsumerHeartbeatKey(ready)).Err())
	t.Cleanup(func() {
		_ = client.Del(context.Background(), ready, processing, taskqueue.ConsumerHeartbeatKey(ready)).Err()
	})

	payload := `{"taskId":"test-recovery-guard"}`
	require.NoError(t, client.LPush(ctx, processing, payload).Err())

	first, err := taskqueue.RegisterConsumer(ctx, client, ready)
	require.NoError(t, err)
	require.NotNil(t, first)
	defer first.Stop(context.Background())
	second, err := taskqueue.RegisterConsumer(ctx, client, ready)
	require.NoError(t, err)
	require.NotNil(t, second)
	defer second.Stop(context.Background())

	recovered, err := taskqueue.RecoverIfIdle(ctx, client, ready, second.ID())
	require.NoError(t, err)
	require.False(t, recovered)
	require.Equal(t, int64(1), client.LLen(ctx, processing).Val())

	// A stale peer no longer blocks recovery.
	require.NoError(t, client.HSet(ctx, taskqueue.ConsumerHeartbeatKey(ready), first.ID(), time.Now().Add(-time.Hour).UnixMilli()).Err())
	recovered, err = taskqueue.RecoverIfIdle(ctx, client, ready, second.ID())
	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, int64(0), client.LLen(ctx, processing).Val())
	require.Equal(t, int64(1), client.LLen(ctx, ready).Val())
}

func TestRedisConsumerGroupRecoveryIgnoresItsOwnSlots(t *testing.T) {
	client, ctx := openTestRedis(t)
	ready := "test:trademind:queue:consumer-group-recovery"
	processing := taskqueue.ProcessingQueue(ready)
	heartbeats := taskqueue.ConsumerHeartbeatKey(ready)
	require.NoError(t, client.Del(ctx, ready, processing, heartbeats).Err())
	t.Cleanup(func() { _ = client.Del(context.Background(), ready, processing, heartbeats).Err() })

	payload := `{"taskId":"test-consumer-group-recovery"}`
	require.NoError(t, client.LPush(ctx, processing, payload).Err())

	group, err := taskqueue.RegisterConsumerGroup(ctx, client, ready)
	require.NoError(t, err)
	require.NotNil(t, group)
	defer group.Stop(context.Background())
	first, err := group.RegisterSlot(ctx)
	require.NoError(t, err)
	defer first.Stop(context.Background())
	second, err := group.RegisterSlot(ctx)
	require.NoError(t, err)
	defer second.Stop(context.Background())

	recovered, err := group.RecoverIfIdle(ctx)
	require.NoError(t, err)
	require.True(t, recovered)
	require.Equal(t, int64(0), client.LLen(ctx, processing).Val())
	require.Equal(t, int64(1), client.LLen(ctx, ready).Val())
}

func TestRedisConsumerGroupSlotStopRequeuesItsReservation(t *testing.T) {
	client, ctx := openTestRedis(t)
	ready := "test:trademind:queue:consumer-group-stop"
	heartbeats := taskqueue.ConsumerHeartbeatKey(ready)
	require.NoError(t, client.Del(ctx, ready, heartbeats).Err())
	t.Cleanup(func() { _ = client.Del(context.Background(), ready, heartbeats).Err() })

	payload := `{"taskId":"test-consumer-group-stop"}`
	require.NoError(t, client.LPush(ctx, ready, payload).Err())
	group, err := taskqueue.RegisterConsumerGroup(ctx, client, ready)
	require.NoError(t, err)
	defer group.Stop(context.Background())
	slot, err := group.RegisterSlot(ctx)
	require.NoError(t, err)

	reserved, err := taskqueue.Reserve(ctx, client, ready, slot.ID(), time.Second)
	require.NoError(t, err)
	require.Equal(t, payload, reserved)
	require.NoError(t, slot.Stop(context.Background()))
	require.Equal(t, int64(1), client.LLen(ctx, ready).Val())
}

func TestRedisConsumerAutomaticallyRecoversAfterQuickRestart(t *testing.T) {
	client, _ := openTestRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	queue := "test:trademind:queue:quick-restart:" + uuid.NewString()
	deadID := uuid.NewString()
	processing := taskqueue.ConsumerProcessingQueue(queue, deadID)
	heartbeats := taskqueue.ConsumerHeartbeatKey(queue)
	payload := `{"taskId":"crashed-before-heartbeat-expiry"}`
	t.Cleanup(func() { _ = client.Del(context.Background(), queue, processing, heartbeats).Err() })
	require.NoError(t, client.LPush(ctx, processing, payload).Err())
	require.NoError(t, client.HSet(ctx, heartbeats, deadID, time.Now().UnixMilli()).Err())

	consumer, err := taskqueue.RegisterConsumer(ctx, client, queue)
	require.NoError(t, err)
	defer consumer.Stop(context.Background())
	recovered, err := taskqueue.RecoverIfIdle(ctx, client, queue, consumer.ID())
	require.NoError(t, err)
	require.False(t, recovered, "a quick restart must initially respect the old heartbeat")
	require.NoError(t, client.HSet(ctx, taskqueue.ConsumerHeartbeatKey(queue), deadID, time.Now().Add(-time.Minute).UnixMilli()).Err())
	require.Eventually(t, func() bool {
		return client.LLen(ctx, queue).Val() == 1 && client.LLen(ctx, processing).Val() == 0
	}, 7*time.Second, 100*time.Millisecond, "recovery must retry after the crashed owner's heartbeat expires")
	require.Equal(t, payload, client.LIndex(ctx, queue, 0).Val())
}

func TestRedisRecoveryReclaimsDeadGroupWithoutTouchingLiveSlots(t *testing.T) {
	client, ctx := openTestRedis(t)
	queue := "test:trademind:queue:live-slots:" + uuid.NewString()
	group, err := taskqueue.RegisterConsumerGroup(ctx, client, queue)
	require.NoError(t, err)
	defer group.Stop(context.Background())
	slot, err := group.RegisterSlot(ctx)
	require.NoError(t, err)
	active := taskqueue.ConsumerProcessingQueue(queue, slot.ID())
	deadOwner := uuid.NewString()
	dead := taskqueue.ConsumerProcessingQueue(queue, deadOwner+"/"+uuid.NewString())
	heartbeats := taskqueue.ConsumerHeartbeatKey(queue)
	t.Cleanup(func() { _ = client.Del(context.Background(), queue, active, dead, heartbeats).Err() })
	require.NoError(t, client.LPush(ctx, queue, "live-work").Err())
	_, err = taskqueue.Reserve(ctx, client, queue, slot.ID(), time.Second)
	require.NoError(t, err)
	require.NoError(t, client.LPush(ctx, dead, "abandoned-work").Err())
	require.NoError(t, client.HSet(ctx, heartbeats, deadOwner, time.Now().Add(-time.Hour).UnixMilli()).Err())

	_, err = group.RecoverIfIdle(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"live-work"}, client.LRange(ctx, active, 0, -1).Val(), "recovery must preserve even its own live slot")
	require.Equal(t, []string{"abandoned-work"}, client.LRange(ctx, queue, 0, -1).Val())
	require.Zero(t, client.LLen(ctx, dead).Val())
}

func TestRedisRequeueDoesNotResurrectRecoveredReservation(t *testing.T) {
	client, ctx := openTestRedis(t)
	queue := "test:trademind:queue:requeue-fencing:" + uuid.NewString()
	consumerID := uuid.NewString()
	processing := taskqueue.ConsumerProcessingQueue(queue, consumerID)
	t.Cleanup(func() { _ = client.Del(context.Background(), queue, processing).Err() })
	require.NoError(t, client.LPush(ctx, processing, "reserved-work").Err())
	require.NoError(t, taskqueue.Recover(ctx, client, queue))
	require.NoError(t, taskqueue.Requeue(ctx, client, queue, consumerID, "reserved-work"))
	require.Equal(t, []string{"reserved-work"}, client.LRange(ctx, queue, 0, -1).Val(), "a fenced old owner cannot enqueue the same recovered reservation twice")
}

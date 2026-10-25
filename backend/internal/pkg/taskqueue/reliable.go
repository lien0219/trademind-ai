package taskqueue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const (
	consumerHeartbeatTTL      = 15 * time.Second
	consumerHeartbeatInterval = 5 * time.Second
	recoveryLockTTL           = 15 * time.Second
)

// ProcessingQueue returns the durable in-flight list paired with a ready queue.
func ProcessingQueue(queue string) string {
	return strings.TrimSpace(queue) + ":processing"
}

// ConsumerProcessingQueue isolates in-flight messages by consumer so one
// process cannot remove another live process's reservations during recovery.
func ConsumerProcessingQueue(queue, consumerID string) string {
	return ProcessingQueue(queue) + ":" + strings.TrimSpace(consumerID)
}

// ConsumerHeartbeatKey identifies the Redis hash used to coordinate startup
// recovery across processes consuming the same queue.
func ConsumerHeartbeatKey(queue string) string {
	return strings.TrimSpace(queue) + ":consumers"
}

func recoveryLockKey(queue string) string {
	return strings.TrimSpace(queue) + ":recovery-lock"
}

// ConsumerLease records a live queue consumer until its context is cancelled
// or Stop is called. Heartbeats prevent a second process from reclaiming work
// that still belongs to a live consumer.
type ConsumerLease struct {
	client *goredis.Client
	queue  string
	id     string
	ctx    context.Context
	stop   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

// ConsumerGroup shares one heartbeat across all slots consuming the same
// queue. Each slot keeps its own processing list so shutdown can return only
// the payloads reserved by that slot while group-level recovery can ignore all
// slots belonging to the current worker process.
type ConsumerGroup struct {
	lease *ConsumerLease
	mu    sync.Mutex
	slots map[string]*ConsumerSlot
	once  sync.Once
}

// ConsumerSlot identifies one concurrent queue slot in a ConsumerGroup.
type ConsumerSlot struct {
	group   *ConsumerGroup
	queue   string
	IDValue string
	once    sync.Once
}

// RegisterConsumerGroup creates one heartbeat shared by all slots in a worker
// group. The group heartbeat is the process-level liveness signal used by
// startup recovery.
func RegisterConsumerGroup(ctx context.Context, client *goredis.Client, queue string) (*ConsumerGroup, error) {
	lease, err := RegisterConsumer(ctx, client, queue)
	if err != nil {
		return nil, err
	}
	return &ConsumerGroup{lease: lease, slots: make(map[string]*ConsumerSlot)}, nil
}

// RegisterSlot allocates an isolated processing list within the group.
func (g *ConsumerGroup) RegisterSlot(_ context.Context) (*ConsumerSlot, error) {
	if g == nil || g.lease == nil {
		return nil, fmt.Errorf("taskqueue: consumer group unavailable")
	}
	slot := &ConsumerSlot{group: g, queue: g.lease.queue, IDValue: g.lease.id + "/" + uuid.NewString()}
	g.mu.Lock()
	if g.slots == nil {
		g.slots = make(map[string]*ConsumerSlot)
	}
	g.slots[slot.IDValue] = slot
	g.mu.Unlock()
	return slot, nil
}

// ID returns the slot's processing-list identity.
func (s *ConsumerSlot) ID() string {
	if s == nil {
		return ""
	}
	return s.IDValue
}

// Stop requeues payloads reserved by this slot and unregisters it from its
// group. It is safe to call more than once.
func (s *ConsumerSlot) Stop(ctx context.Context) error {
	if s == nil || s.group == nil || s.group.lease == nil {
		return nil
	}
	var stopErr error
	s.once.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}
		processing := ConsumerProcessingQueue(s.queue, s.IDValue)
		for {
			_, err := s.group.lease.client.RPopLPush(ctx, processing, s.queue).Result()
			if err == goredis.Nil {
				break
			}
			if err != nil {
				stopErr = err
				break
			}
		}
		s.group.mu.Lock()
		delete(s.group.slots, s.IDValue)
		s.group.mu.Unlock()
	})
	return stopErr
}

// RecoverIfIdle performs startup recovery once for the whole worker group.
// Other processes remain protected by their group heartbeat; slots in this
// process do not block recovery of the group's own abandoned reservations.
func (g *ConsumerGroup) RecoverIfIdle(ctx context.Context) (bool, error) {
	if g == nil || g.lease == nil {
		return false, fmt.Errorf("taskqueue: consumer group unavailable")
	}
	return RecoverIfIdle(ctx, g.lease.client, g.lease.queue, g.lease.ID())
}

// Stop stops all slots before removing the group heartbeat.
func (g *ConsumerGroup) Stop(ctx context.Context) error {
	if g == nil || g.lease == nil {
		return nil
	}
	var stopErr error
	g.once.Do(func() {
		g.mu.Lock()
		slots := make([]*ConsumerSlot, 0, len(g.slots))
		for _, slot := range g.slots {
			slots = append(slots, slot)
		}
		g.mu.Unlock()
		for _, slot := range slots {
			if err := slot.Stop(ctx); err != nil && stopErr == nil {
				stopErr = err
			}
		}
		if err := g.lease.Stop(ctx); err != nil && stopErr == nil {
			stopErr = err
		}
	})
	return stopErr
}

// ID returns the opaque consumer identity stored in Redis.
func (l *ConsumerLease) ID() string {
	if l == nil {
		return ""
	}
	return l.id
}

// RegisterConsumer creates a heartbeat for one queue consumer.
func RegisterConsumer(ctx context.Context, client *goredis.Client, queue string) (*ConsumerLease, error) {
	if client == nil {
		return nil, fmt.Errorf("taskqueue: nil redis client")
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return nil, fmt.Errorf("taskqueue: queue required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	id := uuid.NewString()
	lease := &ConsumerLease{
		client: client,
		queue:  queue,
		id:     id,
		ctx:    ctx,
		stop:   make(chan struct{}),
	}
	if err := lease.heartbeat(context.Background()); err != nil {
		return nil, err
	}
	lease.wg.Add(1)
	go lease.runHeartbeat()
	return lease, nil
}

func (l *ConsumerLease) heartbeat(ctx context.Context) error {
	return l.client.HSet(ctx, ConsumerHeartbeatKey(l.queue), l.id, time.Now().UTC().UnixMilli()).Err()
}

func (l *ConsumerLease) runHeartbeat() {
	defer l.wg.Done()
	ticker := time.NewTicker(consumerHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			hbCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := l.heartbeat(hbCtx); err == nil {
				// Recovery is intentionally periodic. A process that survives a
				// peer crash must eventually reclaim that peer's reservations even
				// when no new process starts after the crash.
				_, _ = RecoverIfIdle(hbCtx, l.client, l.queue, l.id)
			}
			cancel()
		}
	}
}

// Stop removes a consumer heartbeat. It is safe to call more than once.
func (l *ConsumerLease) Stop(ctx context.Context) error {
	if l == nil || l.client == nil {
		return nil
	}
	l.once.Do(func() { close(l.stop) })
	l.wg.Wait()
	if ctx == nil {
		ctx = context.Background()
	}
	processing := ConsumerProcessingQueue(l.queue, l.id)
	for {
		_, err := l.client.RPopLPush(ctx, processing, l.queue).Result()
		if err == goredis.Nil {
			break
		}
		if err != nil {
			return err
		}
	}
	return l.client.HDel(ctx, ConsumerHeartbeatKey(l.queue), l.id).Err()
}

// Reserve atomically moves one payload from ready to processing.
func Reserve(ctx context.Context, client *goredis.Client, queue, consumerID string, timeout time.Duration) (string, error) {
	return client.BRPopLPush(ctx, queue, ConsumerProcessingQueue(queue, consumerID), timeout).Result()
}

// Ack removes one successfully handled payload from processing.
func Ack(ctx context.Context, client *goredis.Client, queue, consumerID, payload string) error {
	return client.LRem(ctx, ConsumerProcessingQueue(queue, consumerID), 1, payload).Err()
}

// Requeue atomically returns a payload to ready and removes its reservation.
func Requeue(ctx context.Context, client *goredis.Client, queue, consumerID, payload string) error {
	_, err := client.Eval(ctx, `
local removed = redis.call('lrem', KEYS[2], 1, ARGV[1])
if removed > 0 then
  return redis.call('lpush', KEYS[1], ARGV[1])
end
return 0
`, []string{queue, ConsumerProcessingQueue(queue, consumerID)}, payload).Result()
	return err
}

// Recover moves reservations left by a crashed worker back to the ready list.
func Recover(ctx context.Context, client *goredis.Client, queue string) error {
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return fmt.Errorf("taskqueue: queue required")
	}
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, ProcessingQueue(queue)+"*", 100).Result()
		if err != nil {
			return err
		}
		for _, processing := range keys {
			for {
				_, err := client.RPopLPush(ctx, processing, queue).Result()
				if err == goredis.Nil {
					break
				}
				if err != nil {
					return err
				}
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

// RecoverOnStartup is a compatibility helper for callers that do not keep a
// long-lived consumer lease. Worker processes should register a lease and call
// RecoverIfIdle with its ID so active reservations remain protected.
func RecoverOnStartup(ctx context.Context, client *goredis.Client, queue string) (bool, error) {
	return RecoverIfIdle(ctx, client, queue, "")
}

// RecoverIfIdle requeues in-flight payloads only when all peer heartbeats are
// stale or absent. A short distributed lock serializes simultaneous startups.
func RecoverIfIdle(ctx context.Context, client *goredis.Client, queue, ownID string) (bool, error) {
	if client == nil {
		return false, fmt.Errorf("taskqueue: nil redis client")
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return false, fmt.Errorf("taskqueue: queue required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lockToken := uuid.NewString()
	lockKey := recoveryLockKey(queue)
	ok, err := client.SetNX(ctx, lockKey, lockToken, recoveryLockTTL).Result()
	if err != nil || !ok {
		return false, err
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = client.Eval(unlockCtx, "if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end", []string{lockKey}, lockToken).Err()
	}()

	values, err := client.HGetAll(ctx, ConsumerHeartbeatKey(queue)).Result()
	if err != nil {
		return false, err
	}
	cutoff := time.Now().UTC().Add(-consumerHeartbeatTTL).UnixMilli()
	liveOwners := make(map[string]struct{})
	for id, raw := range values {
		stamp, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || stamp < cutoff {
			if err := client.HDel(ctx, ConsumerHeartbeatKey(queue), id).Err(); err != nil {
				return false, err
			}
			continue
		}
		if id != strings.TrimSpace(ownID) {
			liveOwners[id] = struct{}{}
		}
	}

	base := ProcessingQueue(queue)
	keys, err := scanProcessingQueues(ctx, client, queue)
	if err != nil {
		return false, err
	}
	recovered := false
	for _, processing := range keys {
		owner, owned := processingOwner(base, processing)
		if owned {
			if owner == strings.TrimSpace(ownID) {
				continue
			}
			if _, ok := liveOwners[owner]; ok {
				continue
			}
		} else if len(liveOwners) > 0 {
			// The legacy unscoped processing list has no owner token. Keep it
			// untouched while another consumer is live rather than stealing a
			// reservation that may belong to that consumer.
			continue
		}
		if err := recoverProcessingQueue(ctx, client, processing, queue); err != nil {
			return false, err
		}
		recovered = true
	}
	return recovered, nil
}

func scanProcessingQueues(ctx context.Context, client *goredis.Client, queue string) ([]string, error) {
	var cursor uint64
	var keys []string
	pattern := ProcessingQueue(queue) + "*"
	for {
		batch, next, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			return keys, nil
		}
	}
}

func processingOwner(base, processing string) (string, bool) {
	if processing == base {
		return "", false
	}
	suffix := strings.TrimPrefix(processing, base+":")
	if suffix == processing || suffix == "" {
		return "", false
	}
	if idx := strings.IndexByte(suffix, '/'); idx >= 0 {
		suffix = suffix[:idx]
	}
	return suffix, suffix != ""
}

func recoverProcessingQueue(ctx context.Context, client *goredis.Client, processing, queue string) error {
	for {
		_, err := client.RPopLPush(ctx, processing, queue).Result()
		if err == goredis.Nil {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

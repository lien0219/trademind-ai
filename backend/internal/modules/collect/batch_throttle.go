package collect

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"
)

func randomDelayMs(minMs, maxMs int) int {
	if minMs < 0 {
		minMs = 0
	}
	if maxMs < minMs {
		maxMs = minMs
	}
	if maxMs == minMs {
		return minMs
	}
	return minMs + rand.IntN(maxMs-minMs+1)
}

// runBatchCollectPreflight applies random delay and optional source concurrency gate for batch tasks.
func (s *Service) runBatchCollectPreflight(ctx context.Context, task *CollectTask) (release func(), err error) {
	release = func() {}
	if s == nil || task == nil || task.BatchID == nil {
		return release, nil
	}
	policy := s.batchPolicyForSource(ctx, task.Source)
	if !policy.ThrottleEnabled() {
		return release, nil
	}

	if policy.DelayMaxMs > 0 {
		delayMs := randomDelayMs(policy.DelayMinMs, policy.DelayMaxMs)
		s.RecordTaskEvent(ctx, task, TaskEventInput{
			EventType:  EventBatchDelayApplied,
			FromStatus: StatusRunning,
			ToStatus:   StatusRunning,
			Message:    "batch random delay before collect",
			PayloadMap: map[string]any{
				"delayMs": delayMs,
				"source":  strings.TrimSpace(task.Source),
			},
		})
		timer := time.NewTimer(time.Duration(delayMs) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	if policy.Concurrency > 0 {
		rel, err := s.acquireBatchSourceSlot(ctx, task.Source, task.ID, policy.Concurrency)
		if err != nil {
			return nil, err
		}
		release = rel
	}
	return release, nil
}

func (s *Service) acquireBatchSourceSlot(ctx context.Context, source string, taskID uuid.UUID, maxConc int) (func(), error) {
	noop := func() {}
	if maxConc < 1 {
		maxConc = 1
	}
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return noop, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	src := strings.ToLower(strings.TrimSpace(source))
	if src == "" {
		src = "unknown"
	}
	tid := taskID.String()
	ttl := 15 * time.Minute
	if s.TaskLeaseTimeoutSeconds > 0 {
		ttl = time.Duration(s.TaskLeaseTimeoutSeconds+60) * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		for i := 0; i < maxConc; i++ {
			key := fmt.Sprintf("collect:batch:%s:gate:%d", src, i)
			ok, err := s.Redis.SetNX(ctx, key, tid, ttl).Result()
			if err != nil {
				return nil, fmt.Errorf("batch gate: %w", err)
			}
			if ok {
				return func() {
					_, _ = s.Redis.Eval(context.Background(), `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
else
  return 0
end`, []string{key}, tid).Result()
				}, nil
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
}

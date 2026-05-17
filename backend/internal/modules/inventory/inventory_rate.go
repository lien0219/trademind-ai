package inventory

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// InventoryRateDefer returns true when the worker should requeue this task without claiming (basic per-platform throttle).
func (s *Service) InventoryRateDefer(ctx context.Context, platform string) (bool, error) {
	enabled, lim := s.inventoryPlatformRateLimit(ctx, platform)
	if !enabled || lim <= 0 || s == nil || s.Redis == nil || s.Redis.Client == nil {
		return false, nil
	}
	key := s.inventoryRateRedisKey(platform, rateMinuteBucket(time.Now()))
	n, err := s.Redis.Get(ctx, key).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return n >= lim, nil
}

// InventoryRateObserveStarted increments the rolling-minute counter after a successful lease claim.
func (s *Service) InventoryRateObserveStarted(ctx context.Context, platform string) {
	enabled, lim := s.inventoryPlatformRateLimit(ctx, platform)
	if !enabled || lim <= 0 || s == nil || s.Redis == nil || s.Redis.Client == nil {
		return
	}
	key := s.inventoryRateRedisKey(platform, rateMinuteBucket(time.Now()))
	_ = s.Redis.Incr(ctx, key).Err()
	_ = s.Redis.Expire(ctx, key, 120*time.Second).Err()
}

func rateMinuteBucket(ts time.Time) int64 {
	return ts.UTC().Unix() / 60
}

func (s *Service) inventoryRateRedisKey(platform string, bucket int64) string {
	pl := strings.TrimSpace(strings.ToLower(platform))
	if pl == "" {
		pl = "unknown"
	}
	return fmt.Sprintf("inv:sync:rl:%s:%d", pl, bucket)
}

func (s *Service) inventoryPlatformRateLimit(ctx context.Context, platform string) (enabled bool, limit int) {
	enabled = true
	pl := strings.TrimSpace(strings.ToLower(platform))
	limit = 60
	switch pl {
	case "amazon":
		limit = 30
	default:
		limit = 60
	}
	if s == nil || s.Settings == nil {
		return enabled, limit
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "inventory")
	if err != nil {
		return enabled, limit
	}
	enabled = inventoryBool(m, "inventory_sync_platform_rate_limit_enabled", true)
	key := "inventory_sync_platform_rate_limit_per_minute_" + pl
	if v := strings.TrimSpace(m[key]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	return enabled, limit
}

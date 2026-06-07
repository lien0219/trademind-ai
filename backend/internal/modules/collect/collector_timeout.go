package collect

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCollectorHTTPTimeout = 60 * time.Second
	maxCollectorHTTPTimeout     = 300 * time.Second
	taobaoTmallCollectOverhead  = 90 * time.Second
	defaultTaobaoGotoTimeoutMs  = 45000
)

func (s *Service) defaultCollectorHTTPTimeout() time.Duration {
	if s != nil && s.CollectorTimeoutSeconds > 0 {
		return time.Duration(s.CollectorTimeoutSeconds) * time.Second
	}
	return defaultCollectorHTTPTimeout
}

func (s *Service) collectorHTTPTimeoutForTask(ctx context.Context, task *CollectTask, opts map[string]any) time.Duration {
	base := s.defaultCollectorHTTPTimeout()
	if task == nil || !isTaobaoTmallCollectSource(task.Source) {
		return base
	}
	gotoMs := taobaoTmallGotoTimeoutMs(ctx, s, opts)
	computed := time.Duration(gotoMs)*time.Millisecond + taobaoTmallCollectOverhead
	if computed < base {
		return base
	}
	if computed > maxCollectorHTTPTimeout {
		return maxCollectorHTTPTimeout
	}
	return computed
}

func taobaoTmallGotoTimeoutMs(ctx context.Context, s *Service, opts map[string]any) int {
	if opts != nil {
		if ms := intFromAny(opts["gotoTimeoutMs"]); ms > 0 {
			return ms
		}
	}
	if s != nil && s.Settings != nil {
		m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
		if err == nil {
			if v := strings.TrimSpace(m["collect_taobao_tmall_timeout_ms"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					return n
				}
			}
			if v := strings.TrimSpace(m["goto_timeout_ms"]); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					return n
				}
			}
		}
	}
	return defaultTaobaoGotoTimeoutMs
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0
		}
		return int(i)
	default:
		return 0
	}
}

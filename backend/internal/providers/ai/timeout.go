package ai

import (
	"strings"
	"time"
)

// chatCompletionTimeout applies settings.ai timeout_sec with a floor for large completions.
func chatCompletionTimeout(plain map[string]string, maxTok int) time.Duration {
	timeout := 120 * time.Second
	if plain != nil {
		if sec := strings.TrimSpace(plain["timeout_sec"]); sec != "" {
			if n, err := parseTimeoutSec(sec); err == nil {
				timeout = n
			}
		}
	}
	floor := 60 * time.Second
	switch {
	case maxTok >= 1500:
		floor = 180 * time.Second
	case maxTok >= 512:
		floor = 120 * time.Second
	}
	if timeout < floor {
		timeout = floor
	}
	return timeout
}

package inventory

import "errors"

// ErrRedisQueueUnavailable indicates Redis LIST operations failed while queue mode is enabled.
var ErrRedisQueueUnavailable = errors.New("Redis queue unavailable")

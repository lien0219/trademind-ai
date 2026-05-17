package productpublish

import "errors"

// ErrRedisQueueUnavailable indicates LIST operations failed while queue mode is enabled.
var ErrRedisQueueUnavailable = errors.New("Redis queue unavailable")

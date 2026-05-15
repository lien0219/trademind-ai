package collect

import "errors"

var (
	// ErrRedisQueueUnavailable is returned when the collect queue cannot accept jobs.
	ErrRedisQueueUnavailable = errors.New("Redis queue unavailable")
	// ErrCollectQueueDisabled is returned when COLLECT_QUEUE_ENABLED is false.
	ErrCollectQueueDisabled = errors.New("collect queue is disabled")
)

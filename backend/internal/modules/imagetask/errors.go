package imagetask

import "errors"

// ErrImageQueueUnavailable is returned when IMAGE_QUEUE_ENABLED but Redis cannot accept jobs.
var ErrImageQueueUnavailable = errors.New("Image queue unavailable")

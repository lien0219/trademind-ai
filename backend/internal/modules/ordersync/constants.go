package ordersync

const (
	TaskTypeOrderSync = "order_sync"
)

const (
	StatusPending        = "pending"
	StatusRunning        = "running"
	StatusPartialSuccess = "partial_success"
	StatusSuccess        = "success"
	StatusFailed         = "failed"
	StatusCancelled      = "cancelled"
)

const (
	ModeManual      = "manual"
	ModeIncremental = "incremental"
	ModeFull        = "full"
)

package customersync

const (
	TaskTypeCustomerMessageSync = "customer_message_sync"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

const (
	ModeManual      = "manual"
	ModeIncremental = "incremental"
	ModeFull        = "full"
)

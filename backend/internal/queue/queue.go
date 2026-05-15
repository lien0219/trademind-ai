package queue

// Name tags logs and metrics for future Redis-backed async workers (AI tasks, etc.).
// Collect jobs use `internal/modules/collect` (Redis LIST + worker); connections via internal/rdb.
const Name = "queue"

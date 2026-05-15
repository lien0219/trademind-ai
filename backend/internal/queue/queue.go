package queue

// Name tags logs and metrics for future Redis-backed async workers (collect, AI tasks).
// Connections are created via internal/rdb; this package will host producers/consumers.
const Name = "queue"

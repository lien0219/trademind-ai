package imagetask

import (
	"context"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

// ImageQueueHealthBlock mirrors image queue metrics for /health.
type ImageQueueHealthBlock struct {
	Enabled        bool   `json:"enabled"`
	Name           string `json:"name"`
	RedisAvailable bool   `json:"redisAvailable"`
	Depth          int64  `json:"depth"`
	WorkerEnabled  bool   `json:"workerEnabled"`
	WorkerRunning  bool   `json:"workerRunning"`
	Concurrency    int    `json:"concurrency"`
}

// ImageTaskStatusCounts holds GROUP BY status counts for image_tasks.
type ImageTaskStatusCounts struct {
	Pending   int64 `json:"pending"`
	Running   int64 `json:"running"`
	Success   int64 `json:"success"`
	Failed    int64 `json:"failed"`
	Cancelled int64 `json:"cancelled"`
}

// MonitorSnapshot is returned by GET /api/v1/image/tasks/monitor.
type MonitorSnapshot struct {
	Queue  ImageQueueHealthBlock `json:"queue"`
	Worker struct {
		Enabled     bool `json:"enabled"`
		Concurrency int  `json:"concurrency"`
		Running     bool `json:"running"`
	} `json:"worker"`
	Tasks ImageTaskStatusCounts `json:"tasks"`
}

// BuildImageQueueHealthBlock builds a small health payload for image_tasks queue.
func BuildImageQueueHealthBlock(ctx context.Context, redis *rdb.Client, queueEnabled bool, queueName string, workerConcurrency int) ImageQueueHealthBlock {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out := ImageQueueHealthBlock{
		Enabled:       queueEnabled,
		Name:          queueName,
		WorkerEnabled: queueEnabled,
		Concurrency:   normalizeImageWorkerConcurrency(workerConcurrency),
		WorkerRunning: ImageWorkersRunning() && queueEnabled,
	}
	if queueName == "" {
		out.Name = "image:tasks"
	}
	if !queueEnabled || redis == nil || redis.Client == nil {
		out.RedisAvailable = false
		out.Depth = 0
		return out
	}
	if err := redis.Ping(ctx).Err(); err != nil {
		out.RedisAvailable = false
		out.Depth = -1
		return out
	}
	out.RedisAvailable = true
	out.Depth = RedisQueueDepth(ctx, redis, out.Name)
	return out
}

// CountTasksByStatus aggregates image_tasks rows by status.
func (s *Service) CountTasksByStatus(ctx context.Context) (ImageTaskStatusCounts, error) {
	var out ImageTaskStatusCounts
	if s == nil || s.DB == nil {
		return out, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	type row struct {
		Status string
		N      int64
	}
	var rows []row
	if err := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Select("status, count(*) as n").
		Group("status").
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, r := range rows {
		switch r.Status {
		case StatusPending:
			out.Pending = r.N
		case StatusRunning:
			out.Running = r.N
		case StatusSuccess:
			out.Success = r.N
		case StatusFailed:
			out.Failed = r.N
		case StatusCancelled:
			out.Cancelled = r.N
		}
	}
	return out, nil
}

// BuildMonitorSnapshot builds the admin monitor JSON.
func (s *Service) BuildMonitorSnapshot(ctx context.Context) (*MonitorSnapshot, error) {
	if s == nil {
		return &MonitorSnapshot{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	q := BuildImageQueueHealthBlock(ctx, s.Redis, s.QueueEnabled, s.QueueName, ImageWorkerConcurrencyConfigured())

	tasks, err := s.CountTasksByStatus(ctx)
	if err != nil {
		return nil, err
	}
	var snap MonitorSnapshot
	snap.Queue = q
	snap.Worker.Enabled = s.QueueEnabled
	snap.Worker.Concurrency = q.Concurrency
	snap.Worker.Running = q.WorkerRunning
	snap.Tasks = tasks
	return &snap, nil
}

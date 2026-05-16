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
	Retrying  int64 `json:"retrying"`
	Success   int64 `json:"success"`
	Failed    int64 `json:"failed"`
	Cancelled int64 `json:"cancelled"`
}

// ImageMonitorRetry summarizes auto-retry configuration and backlog (non-sensitive).
type ImageMonitorRetry struct {
	Enabled               bool   `json:"enabled"`
	MaxRetries            int    `json:"maxRetries"`
	BaseDelaySeconds      int    `json:"baseDelaySeconds"`
	MaxDelaySeconds       int    `json:"maxDelaySeconds"`
	NextRetryDueCount     int    `json:"nextRetryDueCount"`
	OldestRetryingSeconds *int64 `json:"oldestRetryingSeconds,omitempty"`
}

// ImageMonitorRetrying is a compact retrying task row for the monitor dashboard.
type ImageMonitorRetrying struct {
	ID           string  `json:"id"`
	TaskType     string  `json:"taskType"`
	Provider     string  `json:"provider"`
	ProductID    *string `json:"productId,omitempty"`
	RetryCount   int     `json:"retryCount"`
	MaxRetries   int     `json:"maxRetries"`
	NextRetryAt  *string `json:"nextRetryAt,omitempty"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
	UpdatedAt    string  `json:"updatedAt"`
}

// ImageMonitorFailure is a compact failed task row for the monitor dashboard.
type ImageMonitorFailure struct {
	ID           string  `json:"id"`
	TaskType     string  `json:"taskType"`
	Provider     string  `json:"provider"`
	ProductID    *string `json:"productId,omitempty"`
	ErrorMessage string  `json:"errorMessage"`
	UpdatedAt    string  `json:"updatedAt"`
}

// MonitorSnapshot is returned by GET /api/v1/image/tasks/monitor.
type MonitorSnapshot struct {
	Queue  ImageQueueHealthBlock `json:"queue"`
	Worker struct {
		Enabled     bool `json:"enabled"`
		Concurrency int  `json:"concurrency"`
		Running     bool `json:"running"`
	} `json:"worker"`
	Tasks          ImageTaskStatusCounts  `json:"tasks"`
	Retry          ImageMonitorRetry      `json:"retry"`
	RecentRetrying []ImageMonitorRetrying `json:"recentRetrying"`
	RecentFailures []ImageMonitorFailure  `json:"recentFailures"`
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
		case StatusRetrying:
			out.Retrying = r.N
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

	nowUTC := time.Now().UTC()
	var dueN int64
	if err := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", StatusRetrying, nowUTC).
		Count(&dueN).Error; err != nil {
		return nil, err
	}

	cfgMax := s.MaxAutoRetries
	if cfgMax <= 0 {
		cfgMax = 2
	}
	baseD := s.effectiveRetryBaseSec()
	maxD := s.effectiveRetryMaxSec()
	snap.Retry = ImageMonitorRetry{
		Enabled:           s.AutoRetryEnabled && s.QueueEnabled,
		MaxRetries:        cfgMax,
		BaseDelaySeconds:  baseD,
		MaxDelaySeconds:   maxD,
		NextRetryDueCount: int(dueN),
	}

	var retr []ImageTask
	if err := s.DB.WithContext(ctx).
		Where("status = ?", StatusRetrying).
		Find(&retr).Error; err != nil {
		return nil, err
	}
	var oldestRetryWait *int64
	if len(retr) > 0 {
		var maxSec int64
		for i := range retr {
			sec := int64(nowUTC.Sub(retr[i].UpdatedAt.UTC()).Seconds())
			if sec < 0 {
				sec = 0
			}
			if sec > maxSec {
				maxSec = sec
			}
		}
		oldestRetryWait = &maxSec
	}
	snap.Retry.OldestRetryingSeconds = oldestRetryWait

	var recentR []ImageTask
	if err := s.DB.WithContext(ctx).
		Where("status = ?", StatusRetrying).
		Order("updated_at DESC").
		Limit(10).
		Find(&recentR).Error; err != nil {
		return nil, err
	}
	snap.RecentRetrying = make([]ImageMonitorRetrying, 0, len(recentR))
	for i := range recentR {
		t := &recentR[i]
		var pid *string
		if t.ProductID != nil {
			s := t.ProductID.String()
			pid = &s
		}
		var nxt *string
		if t.NextRetryAt != nil {
			v := t.NextRetryAt.UTC().Format(time.RFC3339)
			nxt = &v
		}
		mr := s.effectiveMaxRetries(t)
		snap.RecentRetrying = append(snap.RecentRetrying, ImageMonitorRetrying{
			ID:           t.ID.String(),
			TaskType:     t.TaskType,
			Provider:     t.Provider,
			ProductID:    pid,
			RetryCount:   t.RetryCount,
			MaxRetries:   mr,
			NextRetryAt:  nxt,
			ErrorMessage: t.ErrorMessage,
			UpdatedAt:    t.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	var fails []ImageTask
	if err := s.DB.WithContext(ctx).
		Where("status = ?", StatusFailed).
		Order("updated_at DESC").
		Limit(10).
		Find(&fails).Error; err != nil {
		return nil, err
	}
	snap.RecentFailures = make([]ImageMonitorFailure, 0, len(fails))
	for i := range fails {
		t := &fails[i]
		var pid *string
		if t.ProductID != nil {
			s := t.ProductID.String()
			pid = &s
		}
		snap.RecentFailures = append(snap.RecentFailures, ImageMonitorFailure{
			ID:           t.ID.String(),
			TaskType:     t.TaskType,
			Provider:     t.Provider,
			ProductID:    pid,
			ErrorMessage: t.ErrorMessage,
			UpdatedAt:    t.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	return &snap, nil
}

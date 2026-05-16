package collect

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

func resolveQueueName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "collect:tasks"
	}
	return name
}

// RedisQueueDepth reports whether Redis answers ping + LLEN for the list key; safe when redis is nil (no panic).
func RedisQueueDepth(ctx context.Context, redis *rdb.Client, queueName string) (redisAvailable bool, depth int64) {
	if ctx == nil {
		ctx = context.Background()
	}
	key := resolveQueueName(queueName)
	if redis == nil || redis.Client == nil {
		return false, 0
	}
	if err := redis.Ping(ctx).Err(); err != nil {
		return false, 0
	}
	n, err := redis.LLen(ctx, key).Result()
	if err != nil {
		return false, 0
	}
	return true, n
}

// CollectMonitorQueue is queue observability for GET /collect/monitor.
type CollectMonitorQueue struct {
	Enabled               bool   `json:"enabled"`
	Name                  string `json:"name"`
	RedisAvailable        bool   `json:"redisAvailable"`
	Depth                 int64  `json:"depth"`
	OldestPendingSeconds *int64  `json:"oldestPendingSeconds,omitempty"`
}

// CollectMonitorWorker exposes configured worker settings and in-process running flag.
type CollectMonitorWorker struct {
	Enabled     bool `json:"enabled"`
	Concurrency int  `json:"concurrency"`
	Running     bool `json:"running"`
}

// CollectMonitorTaskAgg counts collect_tasks by status.
type CollectMonitorTaskAgg struct {
	Pending   int `json:"pending"`
	Retrying  int `json:"retrying"`
	Running   int `json:"running"`
	Success   int `json:"success"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

// CollectMonitorBatchAgg counts collect_batches by derived batch.status.
type CollectMonitorBatchAgg struct {
	Running        int `json:"running"`
	PartialSuccess int `json:"partialSuccess"`
	Success        int `json:"success"`
	Failed         int `json:"failed"`
	Cancelled      int `json:"cancelled"`
}

// CollectMonitorFailure is a compact failed task row for the monitor dashboard.
type CollectMonitorFailure struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	SourceURL    string  `json:"sourceUrl"`
	BatchID      *string `json:"batchId,omitempty"`
	ErrorMessage string  `json:"errorMessage"`
	UpdatedAt    string  `json:"updatedAt"`
}

// CollectMonitorCollector summarizes outbound collector connectivity (non-sensitive).
type CollectMonitorCollector struct {
	BaseURL        string `json:"baseUrl"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Reachable      bool   `json:"reachable"`
	Message        string `json:"message"`
}

// CollectMonitorResponse is GET /api/v1/collect/monitor JSON body (wrapped by envelope).
type CollectMonitorResponse struct {
	Queue          CollectMonitorQueue     `json:"queue"`
	Worker         CollectMonitorWorker    `json:"worker"`
	Tasks          CollectMonitorTaskAgg   `json:"tasks"`
	Batches        CollectMonitorBatchAgg  `json:"batches"`
	RecentFailures []CollectMonitorFailure `json:"recentFailures"`
	Collector      CollectMonitorCollector `json:"collector"`
}

type statusCountRow struct {
	Status string `gorm:"column:status"`
	N      int64  `gorm:"column:n"`
}

// GetCollectMonitor aggregates queue, DB, worker state, and collector /health for the admin dashboard.
func (s *Service) GetCollectMonitor(ctx context.Context) (*CollectMonitorResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collect: no db")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	qname := resolveQueueName(s.QueueName)
	redisAvail, depth := RedisQueueDepth(ctx, s.Redis, qname)

	out := &CollectMonitorResponse{
		Queue: CollectMonitorQueue{
			Enabled:        s.QueueEnabled,
			Name:           qname,
			RedisAvailable: redisAvail,
			Depth:          depth,
		},
		Worker: CollectMonitorWorker{
			Enabled:     CollectWorkerQueueEnabled(),
			Concurrency: CollectWorkerConcurrencyConfigured(),
			Running:     CollectWorkersRunning(),
		},
		RecentFailures: []CollectMonitorFailure{},
	}

	var oldest CollectTask
	err := s.DB.WithContext(ctx).
		Where("status IN ?", []string{StatusPending, StatusRetrying}).
		Order("created_at ASC").
		First(&oldest).Error
	if err == nil {
		sec := int64(time.Since(oldest.CreatedAt).Seconds())
		if sec < 0 {
			sec = 0
		}
		out.Queue.OldestPendingSeconds = &sec
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var taskRows []statusCountRow
	if err := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Select("status, COUNT(*) AS n").
		Group("status").
		Scan(&taskRows).Error; err != nil {
		return nil, err
	}
	for _, r := range taskRows {
		switch r.Status {
		case StatusPending:
			out.Tasks.Pending += int(r.N)
		case StatusRetrying:
			out.Tasks.Retrying += int(r.N)
		case StatusRunning:
			out.Tasks.Running += int(r.N)
		case StatusSuccess:
			out.Tasks.Success += int(r.N)
		case StatusFailed:
			out.Tasks.Failed += int(r.N)
		case StatusCancelled:
			out.Tasks.Cancelled += int(r.N)
		}
	}

	var batchRows []statusCountRow
	if err := s.DB.WithContext(ctx).Model(&CollectBatch{}).
		Select("status, COUNT(*) AS n").
		Group("status").
		Scan(&batchRows).Error; err != nil {
		return nil, err
	}
	for _, r := range batchRows {
		switch r.Status {
		case BatchStatusRunning:
			out.Batches.Running += int(r.N)
		case BatchStatusPartialSuccess:
			out.Batches.PartialSuccess += int(r.N)
		case BatchStatusSuccess:
			out.Batches.Success += int(r.N)
		case BatchStatusFailed:
			out.Batches.Failed += int(r.N)
		case BatchStatusCancelled:
			out.Batches.Cancelled += int(r.N)
		}
	}

	var fails []CollectTask
	if err := s.DB.WithContext(ctx).
		Where("status = ?", StatusFailed).
		Order("updated_at DESC").
		Limit(10).
		Find(&fails).Error; err != nil {
		return nil, err
	}
	out.RecentFailures = make([]CollectMonitorFailure, 0, len(fails))
	for i := range fails {
		t := &fails[i]
		var bid *string
		if t.BatchID != nil {
			s := t.BatchID.String()
			bid = &s
		}
		out.RecentFailures = append(out.RecentFailures, CollectMonitorFailure{
			ID:           t.ID.String(),
			Source:       t.Source,
			SourceURL:    t.SourceURL,
			BatchID:      bid,
			ErrorMessage: t.ErrorMessage,
			UpdatedAt:    t.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	timeoutSec := s.CollectorTimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	baseURL := ""
	if s.Client != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(s.Client.BaseURL), "/")
	}
	col := CollectMonitorCollector{
		BaseURL:        baseURL,
		TimeoutSeconds: timeoutSec,
	}
	if s.Client != nil {
		ok, msg := s.Client.ProbeHealth(ctx)
		col.Reachable = ok
		col.Message = msg
	} else {
		col.Reachable = false
		col.Message = "collector client unavailable"
	}
	out.Collector = col

	return out, nil
}

// CollectQueueHealthBlock is embedded into GET /health payloads (no collector probe).
type CollectQueueHealthBlock struct {
	Enabled             bool   `json:"enabled"`
	Name                string `json:"name"`
	RedisAvailable      bool   `json:"redisAvailable"`
	Depth               int64  `json:"depth"`
	WorkerEnabled       bool   `json:"workerEnabled"`
	WorkerConcurrency   int    `json:"workerConcurrency"`
}

// BuildCollectQueueHealthBlock mirrors queue metrics for process health endpoints.
func BuildCollectQueueHealthBlock(ctx context.Context, redis *rdb.Client, queueEnabled bool, queueName string, workerConcurrency int) CollectQueueHealthBlock {
	key := resolveQueueName(queueName)
	avail, depth := RedisQueueDepth(ctx, redis, key)
	return CollectQueueHealthBlock{
		Enabled:           queueEnabled,
		Name:              key,
		RedisAvailable:    avail,
		Depth:             depth,
		WorkerEnabled:     queueEnabled,
		WorkerConcurrency: workerConcurrency,
	}
}

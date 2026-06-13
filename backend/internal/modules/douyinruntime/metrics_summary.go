package douyinruntime

import (
	"context"

	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
)

// MetricsSummaryDTO exposes rolling 24h metrics for the runtime page.
type MetricsSummaryDTO struct {
	douyinmetrics.Summary24h
}

// GetMetricsSummary returns rolling 24h Douyin metrics.
func (s *Service) GetMetricsSummary(_ context.Context) *MetricsSummaryDTO {
	sum := douyinmetrics.GetSummary24h()
	return &MetricsSummaryDTO{Summary24h: sum}
}

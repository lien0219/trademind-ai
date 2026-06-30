package configstatus

import "context"

// DashboardSummary is a compact config health snapshot for the operations dashboard.
type DashboardSummary struct {
	TotalItems      int `json:"totalItems"`
	RiskCount       int `json:"riskCount"`
	IncompleteCount int `json:"incompleteCount"`
	BlockedCount    int `json:"blockedCount"`
}

// DashboardSummary builds a lightweight config status count (no large payloads).
func (s *Service) DashboardSummary(ctx context.Context) (*DashboardSummary, error) {
	ov, err := s.Build(ctx)
	if err != nil || ov == nil {
		return &DashboardSummary{}, err
	}
	out := &DashboardSummary{TotalItems: len(ov.Items)}
	for _, it := range ov.Items {
		switch it.Status {
		case StatusNotConfigured, StatusConfigError, StatusAwaitingCredential, StatusAwaitingPublicURL, StatusAbnormal:
			out.RiskCount++
			out.IncompleteCount++
		case StatusUnsupported, StatusDisabled:
			out.BlockedCount++
		}
	}
	if ov.DemoData.Status == StatusNotConfigured || ov.DemoData.Status == StatusConfigError {
		out.RiskCount++
	}
	return out, nil
}

package douyinshop

import (
	"encoding/json"
	"strings"
	"time"
)

// RecoveryStatus values stored in task output JSON.
const (
	RecoveryStale         = "stale"
	RecoveryResultUnknown = "result_unknown"
	RecoveryRequired      = "recovery_required"
	RecoverySuperseded    = "superseded"
	RecoverySkipped       = "skipped"
)

// TaskRecoveryMeta is persisted in task output for frontend-readable recovery state.
type TaskRecoveryMeta struct {
	RecoveryStatus string `json:"recoveryStatus,omitempty"`
	RetryAttempts  int    `json:"retryAttempts,omitempty"`
	LastErrorCode  string `json:"lastErrorCode,omitempty"`
	UserMessage    string `json:"userMessage,omitempty"`
	TechnicalCode  string `json:"technicalCode,omitempty"`
}

// UserMessageForRecovery maps internal recovery status to user-facing text.
func UserMessageForRecovery(status string) string {
	switch strings.TrimSpace(status) {
	case RecoveryStale:
		return "任务执行时间过长"
	case RecoveryResultUnknown:
		return "平台处理结果暂时无法确认"
	case RecoveryRequired:
		return "需要检查后才能继续"
	case RecoverySuperseded:
		return "已有更新的同步任务，本任务已跳过"
	default:
		return ""
	}
}

// BuildRecoveryOutput merges recovery meta into existing output map.
func BuildRecoveryOutput(existing map[string]any, meta TaskRecoveryMeta) map[string]any {
	out := map[string]any{}
	for k, v := range existing {
		out[k] = v
	}
	if meta.UserMessage == "" && meta.RecoveryStatus != "" {
		meta.UserMessage = UserMessageForRecovery(meta.RecoveryStatus)
	}
	if meta.TechnicalCode == "" && meta.LastErrorCode != "" {
		meta.TechnicalCode = meta.LastErrorCode
	}
	out["recoveryStatus"] = meta.RecoveryStatus
	out["retryAttempts"] = meta.RetryAttempts
	if meta.LastErrorCode != "" {
		out["lastErrorCode"] = meta.LastErrorCode
	}
	if meta.UserMessage != "" {
		out["userMessage"] = meta.UserMessage
	}
	if meta.TechnicalCode != "" {
		out["technicalCode"] = meta.TechnicalCode
	}
	return out
}

// MarshalRecoveryOutput serializes recovery output.
func MarshalRecoveryOutput(existing map[string]any, meta TaskRecoveryMeta) []byte {
	b, _ := json.Marshal(BuildRecoveryOutput(existing, meta))
	return b
}

// IsTaskStale reports whether a running task exceeded its threshold without progress.
func IsTaskStale(startedAt, updatedAt *time.Time, threshold time.Duration, now time.Time) bool {
	if threshold <= 0 {
		return false
	}
	ref := updatedAt
	if ref == nil {
		ref = startedAt
	}
	if ref == nil {
		return false
	}
	return now.Sub(ref.UTC()) > threshold
}

// StaleThresholdForFeature returns configured stale timeout for a feature.
func StaleThresholdForFeature(feature string, timeouts StaleTimeouts) time.Duration {
	switch strings.TrimSpace(feature) {
	case FeatureProductDraft:
		return timeouts.ProductDraftMin
	case FeatureImageUpload:
		return timeouts.ImageUploadMin
	case FeatureOrderSync:
		return timeouts.OrderSyncMin
	case FeatureInventorySync:
		return timeouts.InventorySyncMin
	default:
		return 30 * time.Minute
	}
}

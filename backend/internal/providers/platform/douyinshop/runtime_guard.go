package douyinshop

import (
	"strconv"
	"strings"
	"time"
)

const (
	RuntimeNormal            = "normal"
	RuntimePaused            = "paused"
	RuntimeEmergencyDisabled = "emergency_disabled"

	CodeDouyinPlatformPaused             = "DOUYIN_PLATFORM_PAUSED"
	CodeDouyinPlatformEmergencyDisabled  = "DOUYIN_PLATFORM_EMERGENCY_DISABLED"
	CodeDouyinFeatureDisabled            = "DOUYIN_FEATURE_DISABLED"
	CodeDouyinTaskBlockedByRuntimeStatus = "DOUYIN_TASK_BLOCKED_BY_RUNTIME_STATUS"
	CodeDouyinTaskStale                  = "DOUYIN_TASK_STALE"
	CodeDouyinTaskResultUnknown          = "DOUYIN_TASK_RESULT_UNKNOWN"
	CodeDouyinTaskRecoveryRequired       = "DOUYIN_TASK_RECOVERY_REQUIRED"
	CodeDouyinTaskRecoveryFailed         = "DOUYIN_TASK_RECOVERY_FAILED"

	FeatureProductDraft  = "product_draft"
	FeatureOrderSync     = "order_sync"
	FeatureInventorySync = "inventory_sync"
	FeatureImageUpload   = "image_upload"

	RuntimeBlockedUserMessage = "抖店相关任务目前已暂停，请联系管理员恢复后再重试。"
)

// RuntimeState holds deploy-level Douyin runtime controls.
type RuntimeState struct {
	Status    string
	Reason    string
	ChangedAt *time.Time
}

// StaleTimeouts holds per-task-type stale detection thresholds.
type StaleTimeouts struct {
	ProductDraftMin  time.Duration
	ImageUploadMin   time.Duration
	OrderSyncMin     time.Duration
	InventorySyncMin time.Duration
}

// DefaultStaleTimeouts returns recommended stale thresholds.
func DefaultStaleTimeouts() StaleTimeouts {
	return StaleTimeouts{
		ProductDraftMin:  10 * time.Minute,
		ImageUploadMin:   15 * time.Minute,
		OrderSyncMin:     30 * time.Minute,
		InventorySyncMin: 15 * time.Minute,
	}
}

func parseRuntimeStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case RuntimePaused:
		return RuntimePaused
	case RuntimeEmergencyDisabled:
		return RuntimeEmergencyDisabled
	default:
		return RuntimeNormal
	}
}

func parseStaleMinutes(raw string, fallback time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	if n, err := parsePositiveInt(raw); err == nil && n > 0 {
		return time.Duration(n) * time.Minute
	}
	return fallback
}

func parsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	return n, nil
}

// RuntimeStateFromMergedMap extracts runtime status and stale timeouts from settings.
func RuntimeStateFromMergedMap(m map[string]string) (RuntimeState, StaleTimeouts) {
	def := DefaultStaleTimeouts()
	st := RuntimeState{Status: RuntimeNormal}
	if m == nil {
		return st, def
	}
	st.Status = parseRuntimeStatus(mapGetCI(m, "platform_runtime_status"))
	st.Reason = strings.TrimSpace(mapGetCI(m, "platform_runtime_status_reason"))
	if raw := strings.TrimSpace(mapGetCI(m, "platform_runtime_status_changed_at")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			st.ChangedAt = &t
		}
	}
	def.ProductDraftMin = parseStaleMinutes(mapGetCI(m, "stale_timeout_product_draft_min"), def.ProductDraftMin)
	def.ImageUploadMin = parseStaleMinutes(mapGetCI(m, "stale_timeout_image_upload_min"), def.ImageUploadMin)
	def.OrderSyncMin = parseStaleMinutes(mapGetCI(m, "stale_timeout_order_sync_min"), def.OrderSyncMin)
	def.InventorySyncMin = parseStaleMinutes(mapGetCI(m, "stale_timeout_inventory_sync_min"), def.InventorySyncMin)
	return st, def
}

// WorkerGuardInput combines runtime config and state for worker pre-checks.
type WorkerGuardInput struct {
	Config  RuntimeConfig
	Runtime RuntimeState
	Feature string
	IsWrite bool
}

// CheckWorkerExecution validates feature switches and platform runtime status before worker execution.
func CheckWorkerExecution(in WorkerGuardInput) *Error {
	cfg := in.Config
	rt := in.Runtime
	feature := strings.TrimSpace(in.Feature)

	if !cfg.RealAPIEnabled {
		return NewError(CodeDouyinFeatureDisabled, RuntimeBlockedUserMessage, "", "real_api_enabled is off", "")
	}

	switch rt.Status {
	case RuntimeEmergencyDisabled:
		if in.IsWrite || feature != "" {
			return NewError(CodeDouyinPlatformEmergencyDisabled, RuntimeBlockedUserMessage, "", rt.Reason, "")
		}
	case RuntimePaused:
		if in.IsWrite {
			return NewError(CodeDouyinPlatformPaused, RuntimeBlockedUserMessage, "", rt.Reason, "")
		}
	}

	switch feature {
	case FeatureProductDraft:
		if !cfg.ProductDraftEnabled {
			return NewError(CodeDouyinFeatureDisabled, RuntimeBlockedUserMessage, "", "product_publish_enabled is off", "")
		}
	case FeatureOrderSync:
		if !cfg.OrderSyncEnabled {
			return NewError(CodeDouyinFeatureDisabled, RuntimeBlockedUserMessage, "", "order_sync_enabled is off", "")
		}
	case FeatureInventorySync:
		if !cfg.InventoryEnabled {
			return NewError(CodeDouyinFeatureDisabled, RuntimeBlockedUserMessage, "", "inventory_sync_enabled is off", "")
		}
	case FeatureImageUpload:
		if !cfg.ProductDraftEnabled {
			return NewError(CodeDouyinFeatureDisabled, RuntimeBlockedUserMessage, "", "product_publish_enabled is off", "")
		}
	}

	if rt.Status == RuntimePaused && feature != "" {
		return NewError(CodeDouyinTaskBlockedByRuntimeStatus, RuntimeBlockedUserMessage, "", rt.Reason, "")
	}
	return nil
}

// CheckRuntimeAllowsNewTask returns error if new tasks should not be accepted.
func CheckRuntimeAllowsNewTask(rt RuntimeState) *Error {
	switch rt.Status {
	case RuntimePaused:
		return NewError(CodeDouyinPlatformPaused, RuntimeBlockedUserMessage, "", rt.Reason, "")
	case RuntimeEmergencyDisabled:
		return NewError(CodeDouyinPlatformEmergencyDisabled, RuntimeBlockedUserMessage, "", rt.Reason, "")
	default:
		return nil
	}
}

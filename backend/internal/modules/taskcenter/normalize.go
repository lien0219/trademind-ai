package taskcenter

import (
	"strings"
	"time"
)

type leaseFields struct {
	Status      string
	LockedUntil *time.Time
	LockedBy    *string
	NextRetryAt *time.Time
	UpdatedAt   time.Time
}

func normalizeFromLease(now time.Time, lf leaseFields) string {
	st := strings.TrimSpace(strings.ToLower(lf.Status))
	switch st {
	case "success":
		return NormSuccess
	case "cancelled", "canceled":
		return NormCancelled
	case "pending":
		return NormPending
	case "retrying":
		if lf.NextRetryAt != nil && now.Sub(*lf.NextRetryAt) > staleRetryAfterDrift*time.Minute {
			return NormStale
		}
		return NormRetrying
	case "failed":
		return NormFailed
	case "running":
		if lf.LockedUntil != nil && lf.LockedBy != nil && strings.TrimSpace(*lf.LockedBy) != "" &&
			!lf.LockedUntil.After(now) {
			return NormLeaseExpired
		}
		return NormRunning
	default:
		return st
	}
}

func isFailureFamily(norm string) bool {
	switch norm {
	case NormFailed, NormRetrying, NormStale, NormLeaseExpired:
		return true
	default:
		return false
	}
}

func isResolvedFamily(norm string) bool {
	switch norm {
	case NormSuccess, NormCancelled:
		return true
	default:
		return false
	}
}

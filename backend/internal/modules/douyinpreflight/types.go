package douyinpreflight

import "time"

const (
	statusPassed  = "passed"
	statusWarning = "warning"
	statusFailed  = "failed"

	settingsGroup = "douyin_preflight"
	settingsKey   = "latest_result"
)

// CheckItem is one preflight check result.
type CheckItem struct {
	Key              string         `json:"key"`
	Status           string         `json:"status"`
	Title            string         `json:"title"`
	Message          string         `json:"message"`
	Suggestion       string         `json:"suggestion,omitempty"`
	TechnicalDetails map[string]any `json:"technicalDetails,omitempty"`
}

// Result is the aggregated preflight outcome.
type Result struct {
	Status        string      `json:"status"`
	Checks        []CheckItem `json:"checks"`
	PassedCount   int         `json:"passedCount"`
	WarningCount  int         `json:"warningCount"`
	FailedCount   int         `json:"failedCount"`
	CheckedAt     string      `json:"checkedAt"`
	LiveTest      bool        `json:"liveTest,omitempty"`
	BlockedByReal bool        `json:"blockedByRealCredentials,omitempty"`
}

// RunRequest controls optional live platform probes.
type RunRequest struct {
	LiveTest bool `json:"liveTest"`
}

func aggregateStatus(checks []CheckItem) (string, int, int, int) {
	var passed, warning, failed int
	for _, c := range checks {
		switch c.Status {
		case statusPassed:
			passed++
		case statusWarning:
			warning++
		case statusFailed:
			failed++
		}
	}
	overall := statusPassed
	if failed > 0 {
		overall = statusFailed
	} else if warning > 0 {
		overall = statusWarning
	}
	return overall, passed, warning, failed
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func checkPassed(key, title, msg string, details map[string]any) CheckItem {
	return CheckItem{Key: key, Status: statusPassed, Title: title, Message: msg, TechnicalDetails: details}
}

func checkWarning(key, title, msg, suggestion string, details map[string]any) CheckItem {
	return CheckItem{Key: key, Status: statusWarning, Title: title, Message: msg, Suggestion: suggestion, TechnicalDetails: details}
}

func checkFailed(key, title, msg, suggestion string, details map[string]any) CheckItem {
	return CheckItem{Key: key, Status: statusFailed, Title: title, Message: msg, Suggestion: suggestion, TechnicalDetails: details}
}

package worker

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

// GenerateWorkerID builds collect-{hostname}-{pid}-{shortUUID}.
func GenerateWorkerID(workerType string) string {
	host, _ := os.Hostname()
	host = strings.ReplaceAll(strings.TrimSpace(host), " ", "_")
	if host == "" {
		host = "unknown-host"
	}
	t := strings.TrimSpace(strings.ToLower(workerType))
	if t == "" {
		t = "worker"
	}
	u := uuid.New().String()
	short := u
	if len(u) >= 8 {
		short = u[:8]
	}
	return fmt.Sprintf("%s-%s-%d-%s", t, host, os.Getpid(), short)
}

// GenerateInlineWorkerID is used for synchronous API execution paths (no heartbeat row).
func GenerateInlineWorkerID(workerType string) string {
	return "inline-" + GenerateWorkerID(workerType)
}

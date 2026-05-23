package imagetask

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BuildAIImageObjectKey returns products/{productId}/ai/{taskType}/{yyyy}/{mm}/{uuid}.webp
func BuildAIImageObjectKey(productID *uuid.UUID, taskType string) string {
	now := time.Now().UTC()
	yyyy := now.Format("2006")
	mm := now.Format("01")
	id := uuid.NewString()
	tt := strings.TrimSpace(strings.ToLower(taskType))
	if tt == "" {
		tt = "processed"
	}
	pid := "unknown"
	if productID != nil && *productID != uuid.Nil {
		pid = productID.String()
	}
	return fmt.Sprintf("products/%s/ai/%s/%s/%s/%s.webp", pid, tt, yyyy, mm, id)
}

package lazada

import (
	"fmt"
	"strings"
)

func orderStatusCSV(in map[string]any) string {
	if s := pickStr(in, "order_status", "status"); strings.TrimSpace(s) != "" {
		return s
	}
	if arr, ok := in["statuses"].([]any); ok && len(arr) > 0 {
		parts := make([]string, 0, len(arr))
		for _, v := range arr {
			parts = append(parts, strings.TrimSpace(fmt.Sprint(v)))
		}
		return strings.Join(parts, ",")
	}
	return ""
}

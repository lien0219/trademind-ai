package productpublish

import "strings"

func sanitizeRawErrorMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		low := strings.ToLower(strings.TrimSpace(k))
		if strings.Contains(low, "token") || strings.Contains(low, "secret") {
			out[k] = "****"
			continue
		}
		switch x := v.(type) {
		case string:
			if len(x) > 500 {
				out[k] = x[:500] + "..."
			} else {
				out[k] = x
			}
		default:
			out[k] = v
		}
	}
	return out
}

package imagetask

import "strings"

func processedObjectTag(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "removebg":
		return "removebg"
	case "openai_image":
		return "openai-image"
	default:
		p := strings.TrimSpace(strings.ToLower(provider))
		if p == "" {
			return "processed"
		}
		return strings.ReplaceAll(strings.ReplaceAll(p, "_", "-"), "/", "-")
	}
}

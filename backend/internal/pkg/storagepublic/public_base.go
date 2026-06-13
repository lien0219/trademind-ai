package storagepublic

import (
	"strings"
)

const defaultLocalPublicBase = "/static"

// ResolvePublicBase returns the configured public URL prefix for the active storage kind.
func ResolvePublicBase(m map[string]string) string {
	if m == nil {
		return ""
	}
	kind := strings.ToLower(strings.TrimSpace(m["kind"]))
	if kind == "" {
		kind = "local"
	}
	switch kind {
	case "local":
		pub := strings.TrimSpace(m["public_base"])
		if pub == "" {
			pub = defaultLocalPublicBase
		}
		return strings.TrimRight(pub, "/")
	case "s3", "r2", "minio":
		pub := strings.TrimSpace(firstNonEmpty(m["s3_public_base"], m["public_base"]))
		return strings.TrimRight(pub, "/")
	case "cos":
		pub := strings.TrimSpace(firstNonEmpty(m["cos_public_base"], m["public_base"]))
		return strings.TrimRight(pub, "/")
	case "oss":
		pub := strings.TrimSpace(firstNonEmpty(m["oss_public_base"], m["public_base"]))
		return strings.TrimRight(pub, "/")
	default:
		return strings.TrimRight(strings.TrimSpace(m["public_base"]), "/")
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

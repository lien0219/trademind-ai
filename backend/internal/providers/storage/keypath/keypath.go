package keypath

import (
	"fmt"
	"strings"
)

// NormalizeSafe trims slashes, forbids traversal and empty segments, and returns POSIX-style keys.
func NormalizeSafe(objectKey string) (string, error) {
	key := strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(objectKey), `\`, `/`), "/")
	if key == "" {
		return "", fmt.Errorf("empty object key")
	}
	if strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid object key")
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." {
			return "", fmt.Errorf("invalid object key segment")
		}
	}
	return key, nil
}

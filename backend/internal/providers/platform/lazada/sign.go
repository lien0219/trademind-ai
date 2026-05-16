package lazada

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

const SignMethodSHA256 = "sha256"

// Sign computes Lazada Open Platform HMAC-SHA256 signature (hex uppercase).
// apiPath is the API name only, e.g. "/order/get".
// params must not include "sign"; empty values are omitted from signing per platform rules.
func Sign(apiPath string, params map[string]string, body string, appSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		kk := strings.TrimSpace(k)
		if kk == "" || strings.EqualFold(kk, "sign") {
			continue
		}
		if strings.TrimSpace(params[k]) == "" {
			continue
		}
		keys = append(keys, kk)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(apiPath)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	if body != "" {
		b.WriteString(body)
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(b.String()))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

package douyinshop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

func CanonicalSigningString(params map[string]string, appSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		name := strings.TrimSpace(k)
		if name == "" || name == "sign" || name == "access_token" {
			continue
		}
		keys = append(keys, name)
	}
	sort.Strings(keys)

	secret := strings.TrimSpace(appSecret)
	var b strings.Builder
	b.WriteString(secret)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(secret)
	return b.String()
}

func Sign(params map[string]string, appSecret string) string {
	payload := CanonicalSigningString(params, appSecret)
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(appSecret)))
	_, _ = mac.Write([]byte(payload))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

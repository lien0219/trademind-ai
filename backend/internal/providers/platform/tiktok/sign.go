package tiktok

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

var errMissingSecret = errors.New("tiktok: app_secret required for signing")

// SignOpenAPI produces signature + unix timestamp compatible with TikTok Shop Open Platform.
// access_token/sign/app_secret/token are excluded from concatenated signing material (still expected as query params as required by TikTok).
func SignOpenAPI(path string, appSecret string, params map[string]string, bodyJSON string, tsOverride int64) (signature string, ts int64, err error) {
	if strings.TrimSpace(appSecret) == "" {
		return "", 0, errMissingSecret
	}
	ts = tsOverride
	if ts <= 0 {
		ts = time.Now().Unix()
	}
	work := map[string]string{}
	for k, v := range params {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		work[k] = v
	}
	work["timestamp"] = strconv.FormatInt(ts, 10)

	excludedExact := map[string]bool{"sign": true, "access_token": true, "token": true, "app_secret": true}

	type kv struct {
		k string
		v string
	}
	var pairs []kv
	for k, v := range work {
		if excludedExact[strings.ToLower(strings.TrimSpace(k))] {
			continue
		}
		pairs = append(pairs, kv{k: strings.TrimSpace(k), v: v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return strings.ToLower(pairs[i].k) < strings.ToLower(pairs[j].k)
	})
	input := ""
	for _, p := range pairs {
		input += strings.ToLower(p.k) + p.v
	}
	if bodyJSON == "" {
		bodyJSON = ""
	}
	plain := appSecret + path + input + bodyJSON + appSecret
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(plain))
	return hex.EncodeToString(mac.Sum(nil)), ts, nil
}

package collect

import (
	"encoding/json"
	"net/url"
	"strings"
)

// PinduoduoProfileKey is the dedicated collector persistent profile (not 1688/custom).
const PinduoduoProfileKey = "pinduoduo"

func isPifaPinduoduoURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	return host == "pifa.pinduoduo.com" || strings.HasSuffix(host, ".pifa.pinduoduo.com")
}

func shouldUsePinduoduoDedicatedProfile(sourceURL string, useBrowserProfile bool) bool {
	return useBrowserProfile || isPifaPinduoduoURL(sourceURL)
}

func buildPinduoduoRequestOptions(sourceURL string, useBrowserProfile bool) []byte {
	if !shouldUsePinduoduoDedicatedProfile(sourceURL, useBrowserProfile) {
		return nil
	}
	blob, _ := json.Marshal(map[string]any{
		"useBrowserProfile": true,
		"profileKey":        PinduoduoProfileKey,
	})
	return blob
}

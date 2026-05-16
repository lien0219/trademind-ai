package shopee

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// SignHMAC returns hex HMAC-SHA256(partnerKey, baseString) per Shopee Open API v2.
func SignHMAC(partnerKey, baseString string) string {
	mac := hmac.New(sha256.New, []byte(partnerKey))
	_, _ = mac.Write([]byte(baseString))
	return hex.EncodeToString(mac.Sum(nil))
}

// BaseStringPublic is partner-level signing: partner_id + path + timestamp.
func BaseStringPublic(partnerID int64, apiPath string, timestamp int64) string {
	return strconv.FormatInt(partnerID, 10) + apiPath + strconv.FormatInt(timestamp, 10)
}

// BaseStringShop is shop-level signing: partner_id + path + timestamp + access_token + shop_id.
func BaseStringShop(partnerID int64, apiPath string, timestamp int64, accessToken string, shopID int64) string {
	return strconv.FormatInt(partnerID, 10) + apiPath + strconv.FormatInt(timestamp, 10) + accessToken + strconv.FormatInt(shopID, 10)
}

package tiktok

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// ExchangeAuthCode exchanges an OAuth authorization code for tokens (shop OAuth callback).
func ExchangeAuthCode(ctx context.Context, auth platformp.TestConnectionRequest, code string) (TokenEnvelope, string, string, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return TokenEnvelope{}, "", "", err
	}
	c := http.Client{Timeout: cfg.HTTPTimeout}
	return exchangeAuthCodeHTTP(ctx, c, cfg, code)
}

// RefreshAccessToken refreshes TikTok OAuth tokens using refresh_token grant.
func RefreshAccessToken(ctx context.Context, auth platformp.TestConnectionRequest, refresh string) (TokenEnvelope, string, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return TokenEnvelope{}, "", err
	}
	c := http.Client{Timeout: cfg.HTTPTimeout}
	return refreshTokenHTTP(ctx, c, cfg, refresh)
}

// PrimaryAuthorizedShop returns the first linked shop cipher/profile (best-effort).
func PrimaryAuthorizedShop(ctx context.Context, auth platformp.TestConnectionRequest, accessToken string) (cipher, shopName, extShopID, region, currency string, err error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return "", "", "", "", "", err
	}
	c := http.Client{Timeout: cfg.HTTPTimeout}

	blob, err := signedGET(ctx, c, cfg, cfg.APIShopCipherPath, accessToken, nil)
	if err != nil {
		return "", "", "", "", "", err
	}
	m, err := firstJSONMap(blob)
	if err != nil {
		return "", "", "", "", "", err
	}
	if code, ok := m["code"].(float64); ok && int(code) != 0 {
		msg, _ := m["message"].(string)
		return "", "", "", "", "", fmt.Errorf("%s", strings.TrimSpace(msg))
	}

	var shops []interface{}
	raw := m["data"]
	if dm, ok := raw.(map[string]interface{}); ok {
		if v, ok2 := dm["shops"].([]interface{}); ok2 {
			shops = v
		} else if v, ok2 := dm["shop_list"].([]interface{}); ok2 {
			shops = v
		}
	}
	if len(shops) == 0 {
		return "", "", "", "", "", fmt.Errorf("tiktok authorized shop list empty (check API scopes / configuration)")
	}
	ent, ok := shops[0].(map[string]interface{})
	if !ok {
		return "", "", "", "", "", fmt.Errorf("unexpected shop payload")
	}
	cipher = strField(ent, "cipher", "shop_cipher")
	shopName = strField(ent, "name", "shop_name")
	extShopID = strField(ent, "id", "shop_id")
	region = strField(ent, "region")
	currency = strField(ent, "currency")

	if cipher == "" {
		return "", "", "", "", "", fmt.Errorf("authorized shop cipher missing")
	}
	return cipher, shopName, extShopID, region, currency, nil
}

func strField(m map[string]interface{}, aliases ...string) string {
	for _, k := range aliases {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return strings.TrimSpace(t)
			}
		case float64:
			return strings.TrimSpace(fmt.Sprintf("%.0f", t))
		}
	}
	return ""
}

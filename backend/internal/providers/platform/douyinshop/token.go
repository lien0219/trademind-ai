package douyinshop

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type TokenBundle struct {
	AccessToken           string
	RefreshToken          string
	AccessExpiresAt       *time.Time
	RefreshExpiresAt      *time.Time
	Scopes                []any
	PlatformShopID        string
	ShopName              string
	ShopLogo              string
	ShopStatus            string
	AuthorityID           string
	ShopBizType           string
	RawNonSensitiveFields map[string]any
}

func parseTokenData(now time.Time, data map[string]any) (*TokenBundle, error) {
	access := pickString(data, "access_token", "accessToken")
	if access == "" {
		return nil, fmt.Errorf("douyin token response missing access_token")
	}
	refresh := pickString(data, "refresh_token", "refreshToken")
	accessExp := pickExpiresAt(now, data, "expires_in", "expire_in", "access_token_expire_in", "access_token_expires_in")
	refreshExp := pickExpiresAt(now, data, "refresh_expires_in", "refresh_expire_in", "refresh_token_expire_in", "refresh_token_expires_in")
	if refreshExp == nil && refresh != "" {
		t := now.Add(14 * 24 * time.Hour).UTC()
		refreshExp = &t
	}
	scopes := parseScopes(data)
	shopID := pickString(data, "shop_id", "shopId", "shop_cipher", "shopCipher", "seller_id")
	shopName := pickString(data, "shop_name", "shopName", "name")
	shopStatus := pickString(data, "shop_status", "status", "shopStatus")
	authorityID := pickString(data, "authority_id", "authorityId")
	shopBizType := pickString(data, "shop_biz_type", "shopBizType")
	return &TokenBundle{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
		Scopes:           scopes,
		PlatformShopID:   shopID,
		ShopName:         shopName,
		ShopLogo:         pickString(data, "shop_logo", "logo", "shopLogo"),
		ShopStatus:       shopStatus,
		AuthorityID:      authorityID,
		ShopBizType:      shopBizType,
		RawNonSensitiveFields: map[string]any{
			"shop_id":       shopID,
			"shop_name":     shopName,
			"shop_status":   shopStatus,
			"authority_id":  authorityID,
			"shop_biz_type": shopBizType,
			"saved_at":      now.Format(time.RFC3339),
		},
	}, nil
}

func parseScopes(data map[string]any) []any {
	var scopes []any
	if raw, ok := data["scope"]; ok {
		if s := stringFromAny(raw); s != "" {
			for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
				if p := strings.TrimSpace(part); p != "" {
					scopes = append(scopes, p)
				}
			}
		}
	}
	if raw, ok := data["scopes"].([]any); ok {
		scopes = raw
	}
	if raw, ok := data["auth_scope"].([]any); ok && len(scopes) == 0 {
		scopes = raw
	}
	return scopes
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*TokenBundle, error) {
	var data map[string]any
	if err := c.do(ctx, MethodTokenCreate, map[string]any{"code": strings.TrimSpace(code), "grant_type": "authorization_code"}, "", &data); err != nil {
		return nil, err
	}
	return parseTokenData(c.now(), data)
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenBundle, error) {
	var data map[string]any
	if err := c.do(ctx, MethodTokenRefresh, map[string]any{"refresh_token": strings.TrimSpace(refreshToken), "grant_type": "refresh_token"}, "", &data); err != nil {
		return nil, err
	}
	return parseTokenData(c.now(), data)
}

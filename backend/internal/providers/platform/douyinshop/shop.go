package douyinshop

import (
	"context"
	"strings"
	"time"
)

type ShopInfo struct {
	PlatformShopID   string
	ShopName         string
	ShopLogo         string
	ShopStatus       string
	AuthorityID      string
	ShopBizType      string
	AuthorizedScopes []any
	ExpiresAt        *time.Time
	RefreshExpiresAt *time.Time
	Raw              map[string]any
}

func ShopInfoFromTokenBundle(tok *TokenBundle) *ShopInfo {
	if tok == nil {
		return nil
	}
	raw := tok.RawNonSensitiveFields
	if raw == nil {
		raw = map[string]any{}
	}
	return &ShopInfo{
		PlatformShopID:   strings.TrimSpace(tok.PlatformShopID),
		ShopName:         strings.TrimSpace(tok.ShopName),
		ShopLogo:         strings.TrimSpace(tok.ShopLogo),
		ShopStatus:       strings.TrimSpace(tok.ShopStatus),
		AuthorityID:      strings.TrimSpace(tok.AuthorityID),
		ShopBizType:      strings.TrimSpace(tok.ShopBizType),
		AuthorizedScopes: tok.Scopes,
		ExpiresAt:        tok.AccessExpiresAt,
		RefreshExpiresAt: tok.RefreshExpiresAt,
		Raw:              raw,
	}
}

func (c *Client) GetShopInfo(ctx context.Context, expectedPlatformShopID string) (*ShopInfo, error) {
	tok, err := c.RefreshAccessToken(ctx)
	if err != nil {
		var de *Error
		if AsError(err, &de) {
			switch de.Code {
			case CodeDouyinAuthExpired, CodeDouyinPermissionDenied, CodeDouyinTokenRefreshFailed:
				return nil, de
			}
		}
		return nil, NewError(CodeDouyinShopInfoFailed, "douyin shop info failed", platformCodeOf(err), safeMessageOf(err), requestIDOf(err))
	}
	info := ShopInfoFromTokenBundle(tok)
	if info == nil || (info.PlatformShopID == "" && info.ShopName == "") {
		return nil, NewError(CodeDouyinShopInfoFailed, "douyin shop info failed", "", "missing shop info in token refresh response", "")
	}
	expected := strings.TrimSpace(expectedPlatformShopID)
	if expected != "" && info.PlatformShopID != "" && expected != info.PlatformShopID {
		return nil, NewError(CodeDouyinShopInfoFailed, "douyin shop info failed", "", "shop id mismatch", "")
	}
	return info, nil
}

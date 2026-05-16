package shopee

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// GetShopInfo calls /shop/get_shop_info for connectivity + profile fields.
func GetShopInfo(ctx context.Context, cfg RuntimeConfig, shopID int64, accessToken string) (shopName, region, currency, externalShopID string, err error) {
	r, err := postShop(ctx, cfg, PathGetShopInfo, shopID, accessToken, map[string]any{})
	if err != nil {
		return "", "", "", "", err
	}
	shopName = pickStr(r, "shop_name", "name")
	region = pickStr(r, "region")
	currency = pickStr(r, "currency")
	externalShopID = pickStr(r, "shop_id", "shopid")
	if externalShopID == "" {
		externalShopID = strconv.FormatInt(shopID, 10)
	}
	return strings.TrimSpace(shopName), strings.TrimSpace(region), strings.TrimSpace(currency), strings.TrimSpace(externalShopID), nil
}

func pickStr(m map[string]any, aliases ...string) string {
	for _, k := range aliases {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return strings.TrimSpace(t)
			}
		case float64:
			if t == float64(int64(t)) {
				return strconv.FormatInt(int64(t), 10)
			}
			return strings.TrimSpace(fmt.Sprintf("%.0f", t))
		default:
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

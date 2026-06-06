package shop

import "testing"

func TestDouyinStatePayloadRoundTrip(t *testing.T) {
	raw, err := encodeDouyinStatePayload(douyinOAuthStatePayload{
		Platform: "douyin_shop",
		AdminID:  "admin-1",
		ShopID:   "shop-1",
		Created:  1,
	})
	if err != nil {
		t.Fatalf("encodeDouyinStatePayload() error = %v", err)
	}
	got, err := decodeDouyinStatePayload(raw)
	if err != nil {
		t.Fatalf("decodeDouyinStatePayload() error = %v", err)
	}
	if got.Platform != "douyin_shop" || got.AdminID != "admin-1" || got.ShopID != "shop-1" {
		t.Fatalf("unexpected state payload: %+v", got)
	}
}

func TestDouyinStatePayloadRejectsWrongPlatform(t *testing.T) {
	if _, err := decodeDouyinStatePayload(`{"platform":"tiktok"}`); err == nil {
		t.Fatalf("expected platform mismatch error")
	}
}

func TestDouyinFriendlyMessages(t *testing.T) {
	for _, code := range []string{
		DouyinAppConfigIncomplete,
		DouyinOAuthStateInvalid,
		DouyinTokenExchangeFailed,
		DouyinShopInfoFailed,
		DouyinAuthExpired,
	} {
		if douyinFriendlyMessage(code) == "" || douyinFriendlyMessage(code) == code {
			t.Fatalf("missing friendly message for %s", code)
		}
	}
}

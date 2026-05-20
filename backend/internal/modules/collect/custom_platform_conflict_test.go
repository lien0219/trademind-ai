package collect

import (
	"context"
	"testing"
)

func TestCheckCustomCollectURLConflict(t *testing.T) {
	s := &Service{}
	ctx := context.Background()

	cases := []struct {
		url      string
		wantErr  bool
		wantProv string
	}{
		{"https://detail.1688.com/offer/1.html", true, "1688"},
		{"https://www.aliexpress.com/item/1.html", true, "aliexpress"},
		{"https://item.taobao.com/item.htm?id=1", false, ""},
		{"https://example.com/p/1", false, ""},
	}
	for _, tc := range cases {
		err := s.checkCustomCollectURLConflict(ctx, tc.url)
		if tc.wantErr {
			var c *CustomCollectProviderConflict
			if err == nil {
				t.Fatalf("url=%q expected conflict", tc.url)
			}
			if !asConflict(err, &c) || c.RecommendedProvider != tc.wantProv {
				t.Fatalf("url=%q err=%v want provider %q", tc.url, err, tc.wantProv)
			}
			continue
		}
		if err != nil {
			t.Fatalf("url=%q unexpected err: %v", tc.url, err)
		}
	}
}

func asConflict(err error, out **CustomCollectProviderConflict) bool {
	c, ok := err.(*CustomCollectProviderConflict)
	if !ok || c == nil {
		return false
	}
	*out = c
	return true
}

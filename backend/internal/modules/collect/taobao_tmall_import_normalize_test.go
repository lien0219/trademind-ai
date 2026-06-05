package collect

import "testing"

func TestNormalizeTaobaoTmallImageURL(t *testing.T) {
	raw := "https://gw.alicdn.com/imgextra/O1CN01eYJpsR1GureqZzthW_!!2218895890683-0-picasso.jpg_q50.jpg_.jpg"
	want := "https://gw.alicdn.com/imgextra/O1CN01eYJpsR1GureqZzthW_!!2218895890683-0-picasso.jpg_q50.jpg"
	if got := normalizeTaobaoTmallImageURL(raw); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := normalizeTaobaoTmallImageURL("https://g.alicdn.com/s.gif"); got != "" {
		t.Fatalf("placeholder should be empty, got %q", got)
	}
}

package safefields

import (
	"strings"
	"testing"
)

func TestRedactMapNested(t *testing.T) {
	in := map[string]any{
		"access_token": "secret-at",
		"nested": map[string]any{
			"Refresh_Token": "secret-rt",
			"items": []any{
				map[string]any{"mobile": "13800138000"},
			},
		},
	}
	out := RedactMap(in)
	if out["access_token"] != "****" {
		t.Fatal("top level token")
	}
	nested := out["nested"].(map[string]any)
	if nested["Refresh_Token"] != "****" {
		t.Fatal("nested token case insensitive")
	}
	items := nested["items"].([]any)
	row := items[0].(map[string]any)
	if row["mobile"] != "****" {
		t.Fatal("nested array mobile")
	}
}

func TestRedactString(t *testing.T) {
	s := RedactString("failed: access_token=abc123")
	if !strings.Contains(s, "redacted") {
		t.Fatalf("got %q", s)
	}
}

func TestRedactURLQuery(t *testing.T) {
	u := RedactURL("https://api.example.com?access_token=abc&foo=bar")
	if strings.Contains(u, "abc") {
		t.Fatalf("token leaked: %s", u)
	}
	if !strings.Contains(u, "foo=bar") {
		t.Fatal("non-sensitive preserved")
	}
}

func TestRedactHeaders(t *testing.T) {
	h := RedactHeaders(map[string]string{"Authorization": "Bearer x", "X-Request-Id": "r1"})
	if h["Authorization"] != "****" {
		t.Fatal("auth header")
	}
	if h["X-Request-Id"] != "r1" {
		t.Fatal("safe header")
	}
}

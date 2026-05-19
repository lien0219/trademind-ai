package aimodelparse

import (
	"testing"
)

func TestParseTitleOptimize_StrictJSON(t *testing.T) {
	out, err := ParseTitleOptimize(`{"optimizedTitle":"Hello","keywords":["a"],"reason":"ok"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out.OptimizedTitle != "Hello" || len(out.Keywords) != 1 || out.Reason != "ok" {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestParseTitleOptimize_SnakeCase(t *testing.T) {
	out, err := ParseTitleOptimize(`{"optimized_title":"Snake","keywords":"kw1, kw2","reason":"r"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out.OptimizedTitle != "Snake" || len(out.Keywords) != 2 {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestParseTitleOptimize_CodeFenceAndPrefix(t *testing.T) {
	raw := `Here is the result:
` + "```json\n" + `{"optimizedTitle":"Fenced","keywords":[],"reason":"x"}` + "\n```"
	out, err := ParseTitleOptimize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.OptimizedTitle != "Fenced" {
		t.Fatalf("got %+v", out)
	}
}

func TestParseTitleOptimize_ThinkBlock(t *testing.T) {
	raw := "<" + "think" + ">internal reasoning</" + "think" + ">\n" + `{"optimized_title":"AfterThink","keywords":[],"reason":""}`
	out, err := ParseTitleOptimize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.OptimizedTitle != "AfterThink" {
		t.Fatalf("got %+v", out)
	}
}

func TestParseTitleOptimize_MissingTitle(t *testing.T) {
	_, err := ParseTitleOptimize(`{"keywords":[],"reason":"no title"}`)
	if err == nil {
		t.Fatal("expected error")
	}
}

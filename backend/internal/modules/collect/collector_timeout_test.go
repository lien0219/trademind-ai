package collect

import (
	"context"
	"testing"
	"time"
)

func TestCollectorHTTPTimeoutForTask_TaobaoTmallUsesExtendedTimeout(t *testing.T) {
	s := &Service{CollectorTimeoutSeconds: 60}
	task := &CollectTask{Source: "taobao_tmall"}
	got := s.collectorHTTPTimeoutForTask(context.Background(), task, map[string]any{
		"gotoTimeoutMs": 45000,
	})
	want := 135 * time.Second
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCollectorHTTPTimeoutForTask_NonTaobaoUsesBase(t *testing.T) {
	s := &Service{CollectorTimeoutSeconds: 60}
	task := &CollectTask{Source: "1688"}
	got := s.collectorHTTPTimeoutForTask(context.Background(), task, nil)
	if got != 60*time.Second {
		t.Fatalf("got %v want 60s", got)
	}
}

func TestCollectorHTTPTimeoutForTask_TaobaoRespectsMaxCap(t *testing.T) {
	s := &Service{CollectorTimeoutSeconds: 60}
	task := &CollectTask{Source: "taobao_tmall"}
	got := s.collectorHTTPTimeoutForTask(context.Background(), task, map[string]any{
		"gotoTimeoutMs": 300000,
	})
	if got != maxCollectorHTTPTimeout {
		t.Fatalf("got %v want cap %v", got, maxCollectorHTTPTimeout)
	}
}

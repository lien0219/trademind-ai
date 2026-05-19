package ai

import (
	"testing"
	"time"
)

func TestChatCompletionTimeout_RespectsFloorForLargeMaxTokens(t *testing.T) {
	plain := map[string]string{"timeout_sec": "60"}
	got := chatCompletionTimeout(plain, 1024)
	if got != 120*time.Second {
		t.Fatalf("got %v want 120s", got)
	}
}

func TestChatCompletionTimeout_SmallPingUsesConfigured(t *testing.T) {
	plain := map[string]string{"timeout_sec": "60"}
	got := chatCompletionTimeout(plain, 1)
	if got != 60*time.Second {
		t.Fatalf("got %v want 60s", got)
	}
}

func TestChatCompletionTimeout_ConfigAboveFloor(t *testing.T) {
	plain := map[string]string{"timeout_sec": "300"}
	got := chatCompletionTimeout(plain, 2048)
	if got != 300*time.Second {
		t.Fatalf("got %v want 300s", got)
	}
}

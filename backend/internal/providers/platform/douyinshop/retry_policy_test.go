package douyinshop

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvaluateRetryRateLimited(t *testing.T) {
	err := NewError(CodeDouyinRateLimited, "rate limited", "", "", "")
	dec := EvaluateRetry(err, 1, DefaultRetryPolicy(), 0)
	if !dec.Retryable {
		t.Fatal("expected retryable")
	}
}

func TestEvaluateRetryPermissionDenied(t *testing.T) {
	err := NewError(CodeDouyinPermissionDenied, "denied", "", "", "")
	dec := EvaluateRetry(err, 1, DefaultRetryPolicy(), 0)
	if dec.Retryable {
		t.Fatal("permission error must not retry")
	}
}

func TestEvaluateRetryMaxAttempts(t *testing.T) {
	err := NewError(CodeDouyinRequestTimeout, "timeout", "", "", "")
	dec := EvaluateRetry(err, 3, DefaultRetryPolicy(), 0)
	if dec.Retryable {
		t.Fatal("should stop at max attempts")
	}
}

func TestExecuteWithRetryStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, err := ExecuteWithRetry(ctx, DefaultRetryPolicy(), func(ctx context.Context, attempt int) error {
		calls++
		return NewError(CodeDouyinRequestTimeout, "timeout", "", "", "")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancel, got %v calls=%d", err, calls)
	}
	if calls != 0 {
		t.Fatalf("should not call operation after cancel")
	}
}

func TestExecuteWithRetryRespectsRetryAfter(t *testing.T) {
	policy := DefaultRetryPolicy()
	start := time.Now()
	attempts := 0
	_, err := ExecuteWithRetry(context.Background(), policy, func(ctx context.Context, attempt int) error {
		attempts++
		if attempt < 2 {
			return NewError(CodeDouyinRateLimited, "rate", "", "", "")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d", attempts)
	}
	if time.Since(start) < 400*time.Millisecond {
		t.Fatalf("expected backoff delay")
	}
}

func TestHTTPStatusRetryable5xx(t *testing.T) {
	if !HTTPStatusRetryable(http.StatusBadGateway) {
		t.Fatal("502 should retry")
	}
	if HTTPStatusRetryable(http.StatusBadRequest) {
		t.Fatal("400 should not retry")
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	d := ParseRetryAfter("3")
	if d != 3*time.Second {
		t.Fatalf("got %v", d)
	}
}

func TestTokenRefreshSingleflight(t *testing.T) {
	var refreshCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&refreshCount, 1)
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":10000,"data":{"access_token":"new-at","refresh_token":"new-rt","expires_in":7200,"refresh_token_expires_in":86400}}`))
	}))
	defer srv.Close()

	cfg := RuntimeConfig{
		AppKey: "k", AppSecret: "s", RedirectURI: "https://cb.example/oauth",
		APIBaseURL: srv.URL, HTTPTimeout: 5 * time.Second,
	}
	now := time.Now().UTC().Add(-2 * time.Hour)
	exp := now.Add(-time.Hour)
	client := &Client{
		ShopID:                "shop-1",
		Config:                cfg,
		RefreshTokenValue:     "rt-old",
		AccessTokenExpiresAt:  &exp,
		RefreshTokenExpiresAt: func() *time.Time { t := now.Add(24 * time.Hour); return &t }(),
	}

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := WithShopID(context.Background(), "shop-1")
			_, err := client.EnsureFreshAccessSingleflight(ctx)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("refresh err: %v", err)
		}
	}
	if atomic.LoadInt32(&refreshCount) != 1 {
		t.Fatalf("expected 1 refresh call, got %d", refreshCount)
	}
}

func TestCheckWorkerExecutionPaused(t *testing.T) {
	err := CheckWorkerExecution(WorkerGuardInput{
		Config:  RuntimeConfig{RealAPIEnabled: true, ProductDraftEnabled: true, WriteOperationsEnabled: true},
		Runtime: RuntimeState{Status: RuntimePaused},
		Feature: FeatureProductDraft,
		IsWrite: true,
	})
	if err == nil || err.Code != CodeDouyinPlatformPaused {
		t.Fatalf("got %v", err)
	}
}

func TestCheckWorkerExecutionFeatureDisabled(t *testing.T) {
	err := CheckWorkerExecution(WorkerGuardInput{
		Config:  RuntimeConfig{RealAPIEnabled: false},
		Runtime: RuntimeState{Status: RuntimeNormal},
		Feature: FeatureOrderSync,
		IsWrite: true,
	})
	if err == nil || err.Code != CodeDouyinFeatureDisabled {
		t.Fatalf("got %v", err)
	}
}

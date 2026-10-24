package tiktok

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type refreshTestBridge struct {
	persistErr error
	statusErr  error
}

func (b refreshTestBridge) PersistOAuthTokenRefresh(context.Context, uuid.UUID, string, string, *time.Time, *time.Time) error {
	return b.persistErr
}

func (b refreshTestBridge) SetShopAuthStatus(context.Context, uuid.UUID, string) error {
	return b.statusErr
}

func (b refreshTestBridge) TikTokGlobalSettings(context.Context) (map[string]string, error) {
	return nil, nil
}

func (b refreshTestBridge) TikTokPublishSettings(context.Context) (map[string]string, error) {
	return nil, nil
}

func TestEnsureFreshAccessReturnsPersistenceError(t *testing.T) {
	previous := bridges
	t.Cleanup(func() { bridges = previous })
	wantErr := errors.New("persist refresh failed")
	bridges = refreshTestBridge{persistErr: wantErr}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/token/refresh" {
			t.Fatalf("unexpected refresh path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"code\":0,\"data\":{\"access_token\":\"new-access\",\"refresh_token\":\"new-refresh\",\"expires_in\":3600}}"))
	}))
	t.Cleanup(server.Close)
	auth := platformp.TestConnectionRequest{
		RefreshToken: "old-refresh",
		TokenVersion: 7,
		Extra: map[string]string{
			"auth_base_url": server.URL,
			"api_base_url":  server.URL,
			"app_key":       "test-key",
			"app_secret":    "test-secret",
			"redirect_uri":  server.URL + "/callback",
			"api_version":   "202309",
			"timeout_sec":   "5",
		},
	}

	access, _, err := ensureFreshAccess(context.Background(), uuid.New(), auth)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected persistence error, got access=%q err=%v", access, err)
	}
	if access != "" {
		t.Fatalf("expected no access token after persistence failure, got %q", access)
	}
}

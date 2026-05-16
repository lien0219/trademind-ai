package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenBundle is LWA token response (never log refresh_token).
type TokenBundle struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  *time.Time
	RefreshExpiresAt *time.Time
}

type lwaTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func postFormToken(ctx context.Context, cfg RuntimeConfig, form url.Values) (*TokenBundle, error) {
	tu := strings.TrimSpace(cfg.LWATokenURL)
	if tu == "" {
		return nil, fmt.Errorf("amazon: missing lwa_token_url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tu, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("amazon: lwa token request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("amazon: LWA rate limited (429)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("amazon: LWA token http %d", resp.StatusCode)
	}
	var tr lwaTokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("amazon: lwa token parse: %w", err)
	}
	if tr.Error != "" {
		msg := tr.Error
		if tr.ErrorDescription != "" {
			msg += ": " + tr.ErrorDescription
		}
		return nil, fmt.Errorf("amazon: LWA error %s", msg)
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		return nil, fmt.Errorf("amazon: LWA missing access_token")
	}
	var exp *time.Time
	if tr.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(tr.ExpiresIn) * time.Second)
		exp = &t
	}
	out := &TokenBundle{
		AccessToken:     tr.AccessToken,
		RefreshToken:    tr.RefreshToken,
		AccessExpiresAt: exp,
	}
	return out, nil
}

// ExchangeAuthCode trades authorization code for tokens.
func ExchangeAuthCode(ctx context.Context, cfg RuntimeConfig, code, redirectURI string) (*TokenBundle, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("amazon: authorization code required")
	}
	rd := strings.TrimSpace(redirectURI)
	if rd == "" {
		rd = cfg.RedirectURI
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("redirect_uri", rd)
	return postFormToken(ctx, cfg, form)
}

// RefreshAccessToken refreshes LWA access token.
func RefreshAccessToken(ctx context.Context, cfg RuntimeConfig, refreshToken string) (*TokenBundle, error) {
	ref := strings.TrimSpace(refreshToken)
	if ref == "" {
		return nil, fmt.Errorf("amazon: refresh_token required")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", ref)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	return postFormToken(ctx, cfg, form)
}

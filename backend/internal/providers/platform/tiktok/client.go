package tiktok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func doGET(ctx context.Context, c http.Client, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return b, res.StatusCode, nil
}

func doPOSTJSON(ctx context.Context, c http.Client, rawURL string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return b, res.StatusCode, nil
}

func firstJSONMap(blob []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(blob, &m); err != nil {
		return nil, err
	}
	return m, nil
}

type TokenEnvelope struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  *time.Time
	RefreshExpiresAt *time.Time
}

func tokenEnvelopeFromOAuth(body []byte) (tok TokenEnvelope, sellerName string, sellerRegion string, err error) {
	m, err := firstJSONMap(body)
	if err != nil {
		return tok, "", "", err
	}
	var inner map[string]interface{}
	dataAny, ok := m["data"]
	if ok && dataAny != nil {
		if dm, ok2 := dataAny.(map[string]interface{}); ok2 {
			inner = dm
		}
	}
	if inner == nil {
		inner = m
	}
	tok.AccessToken, _ = inner["access_token"].(string)
	tok.RefreshToken, _ = inner["refresh_token"].(string)
	sellerName, _ = inner["seller_name"].(string)
	sellerRegion, _ = inner["seller_base_region"].(string)
	if ae, ok := inner["access_token_expire_in"].(float64); ok {
		sec := int64(ae)
		t := time.Unix(sec, 0).UTC()
		tok.AccessExpiresAt = &t
	}
	if re, ok := inner["refresh_token_expire_in"].(float64); ok {
		sec := int64(re)
		t := time.Unix(sec, 0).UTC()
		tok.RefreshExpiresAt = &t
	}
	if tok.AccessToken == "" {
		msg, _ := m["message"].(string)
		if msg == "" {
			msg = "token response missing access_token"
		}
		code := ""
		switch c := m["code"].(type) {
		case float64:
			code = fmt.Sprintf("%.0f", c)
		case string:
			code = c
		default:
		}
		if code != "" {
			msg = fmt.Sprintf("tiktok oauth code=%s %s", code, msg)
		}
		return tok, sellerName, sellerRegion, fmt.Errorf("%s", msg)
	}
	return tok, sellerName, sellerRegion, nil
}

func exchangeAuthCodeHTTP(ctx context.Context, c http.Client, cfg RuntimeConfig, code string) (TokenEnvelope, string, string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return TokenEnvelope{}, "", "", fmt.Errorf("auth code required")
	}
	uu, err := url.Parse(cfg.OAuthTokenGetURL)
	if err != nil {
		return TokenEnvelope{}, "", "", err
	}
	q := url.Values{}
	q.Set("app_key", cfg.AppKey)
	q.Set("app_secret", cfg.AppSecret)
	q.Set("auth_code", code)
	q.Set("grant_type", "authorized_code")
	uu.RawQuery = q.Encode()
	b, _, err := doGET(ctx, c, uu.String())
	if err != nil {
		return TokenEnvelope{}, "", "", err
	}
	tp, nm, rg, err := tokenEnvelopeFromOAuth(b)
	return tp, nm, rg, err
}

func refreshTokenHTTP(ctx context.Context, c http.Client, cfg RuntimeConfig, refresh string) (TokenEnvelope, string, error) {
	refresh = strings.TrimSpace(refresh)
	if refresh == "" {
		return TokenEnvelope{}, "", fmt.Errorf("missing refresh_token")
	}
	uu, err := url.Parse(cfg.OAuthTokenRefreshURL)
	if err != nil {
		return TokenEnvelope{}, "", err
	}
	q := url.Values{}
	q.Set("app_key", cfg.AppKey)
	q.Set("app_secret", cfg.AppSecret)
	q.Set("refresh_token", refresh)
	q.Set("grant_type", "refresh_token")
	uu.RawQuery = q.Encode()
	b, _, err := doGET(ctx, c, uu.String())
	if err != nil {
		return TokenEnvelope{}, "", err
	}
	tp, _, rg, err := tokenEnvelopeFromOAuth(b)
	return tp, rg, err
}

func signedPOSTJSON(ctx context.Context, c http.Client, cfg RuntimeConfig, path string, accessToken string, body map[string]interface{}) ([]byte, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("missing access_token")
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	q := map[string]string{
		"app_key":      cfg.AppKey,
		"access_token": accessToken,
		"version":      cfg.APIVersion,
	}
	if strings.TrimSpace(cfg.ShopCipher) != "" {
		q["shop_cipher"] = cfg.ShopCipher
	}
	sig, ts, err := SignOpenAPI(path, cfg.AppSecret, q, string(bodyJSON), 0)
	if err != nil {
		return nil, err
	}
	q["sign"] = sig
	q["timestamp"] = fmt.Sprintf("%d", ts)
	u := strings.TrimSuffix(cfg.OpenAPIHost, "/") + path
	uv, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	vals := url.Values{}
	for k, v := range q {
		vals.Set(k, v)
	}
	uv.RawQuery = vals.Encode()
	b, _, err := doPOSTJSON(ctx, c, uv.String(), bodyJSON)
	return b, err
}

func signedGET(ctx context.Context, c http.Client, cfg RuntimeConfig, path string, accessToken string, extra map[string]string) ([]byte, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("missing access_token")
	}
	q := map[string]string{
		"app_key":      cfg.AppKey,
		"access_token": accessToken,
		"version":      cfg.APIVersion,
	}
	if strings.TrimSpace(cfg.ShopCipher) != "" {
		q["shop_cipher"] = cfg.ShopCipher
	}
	for k, v := range extra {
		if strings.TrimSpace(v) != "" {
			q[k] = v
		}
	}
	sig, ts, err := SignOpenAPI(path, cfg.AppSecret, q, "", 0)
	if err != nil {
		return nil, err
	}
	q["sign"] = sig
	q["timestamp"] = fmt.Sprintf("%d", ts)
	u := strings.TrimSuffix(cfg.OpenAPIHost, "/") + path
	uv, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	vals := url.Values{}
	for k, v := range q {
		vals.Set(k, v)
	}
	uv.RawQuery = vals.Encode()
	b, _, err := doGET(ctx, c, uv.String())
	return b, err
}

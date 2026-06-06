package douyinshop

import "testing"

func validConfigMap() map[string]string {
	return map[string]string{
		"app_key":                 "ak",
		"app_secret":              "secret",
		"service_id":              "svc_123",
		"redirect_uri":            "https://admin.example.com/api/v1/shops/callback",
		"environment":             "production",
		"timeout_sec":             "30",
		"real_api_enabled":        "true",
		"order_sync_enabled":      "false",
		"inventory_sync_enabled":  "false",
		"product_publish_enabled": "false",
	}
}

func TestRuntimeFromMergedMapValid(t *testing.T) {
	cfg, err := RuntimeFromMergedMap(validConfigMap())
	if err != nil {
		t.Fatalf("RuntimeFromMergedMap() error = %v", err)
	}
	if cfg.AppKey != "ak" || cfg.AppSecret != "secret" || cfg.ServiceID != "svc_123" || cfg.Environment != "production" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !cfg.RealAPIEnabled {
		t.Fatalf("expected real api flag true")
	}
}

func TestRuntimeFromMergedMapMissingSecret(t *testing.T) {
	m := validConfigMap()
	m["app_secret"] = ""
	if _, err := RuntimeFromMergedMap(m); err == nil {
		t.Fatalf("expected missing secret error")
	}
}

func TestRuntimeFromMergedMapInvalidEnvironment(t *testing.T) {
	m := validConfigMap()
	m["environment"] = "dev"
	if _, err := RuntimeFromMergedMap(m); err == nil {
		t.Fatalf("expected invalid environment error")
	}
}

func TestRuntimeFromMergedMapInvalidRedirectURI(t *testing.T) {
	m := validConfigMap()
	m["redirect_uri"] = "not-a-url"
	if _, err := RuntimeFromMergedMap(m); err == nil {
		t.Fatalf("expected invalid redirect uri error")
	}
}

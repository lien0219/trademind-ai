package tenantconfig

import (
	"encoding/json"
	"testing"
)

func TestSettingsStrictSchema(t *testing.T) {
	tests := []struct {
		module string
		raw    string
		good   bool
	}{
		{ModuleGeneral, `{"language":"zh-CN","timezone":"Asia/Shanghai","currency":"CNY"}`, true},
		{ModuleGeneral, `{"language":"zh-TW","timezone":"Asia/Shanghai","currency":"CNY"}`, true},
		{ModuleGeneral, `{"language":"ko-KR","timezone":"Asia/Tokyo","currency":"USD"}`, true},
		{ModuleGeneral, `{"language":"th-TH","timezone":"UTC","currency":"USD"}`, true},
		{ModuleAI, `{"defaultLanguage":"ja-JP"}`, true},
		{ModuleAI, `{"defaultLanguage":"zh-TW"}`, true},
		{ModuleAI, `{"defaultLanguage":"ko-KR"}`, true},
		{ModuleAI, `{"defaultLanguage":"th-TH"}`, true},
		{ModuleAI, `{"defaultLanguage":"unexpected"}`, false},
		{ModuleCollect, `{"allowedSources":["1688"],"maxBatchURLs":5}`, true},
		{ModuleCollect, `{"allowedSources":["1688","1688"],"maxBatchURLs":5}`, false},
		{ModuleCollect, `{"allowedSources":[],"maxBatchURLs":101}`, false},
		{ModuleCollect, `{"allowedSources":[],"maxBatchURLs":0,"apiKey":"x"}`, false},
		{ModuleGeneral, `{"language":"zh-CN","timezone":"Asia/Shanghai","currency":"CNY","__proto__":{}}`, false},
		{ModuleInventory, `{"lowStockThreshold":10}`, true},
		{ModuleInventory, `{"lowStockThreshold":-1}`, false},
		{ModuleNotifications, `{"businessAlertsEnabled":true}`, true},
		{ModuleNotifications, `{"businessAlertsEnabled":true,"mailRecipients":["ops@example.com"]}`, true},
		{ModuleNotifications, `{"businessAlertsEnabled":true,"mailRecipients":["name <ops@example.com>"]}`, false},
		{ModuleNotifications, `{"businessAlertsEnabled":true,"mailRecipients":["ops@example.com","OPS@example.com"]}`, false},
		{ModuleOrders, `{"defaultCurrency":"EUR"}`, true},
		{ModuleOrders, `{"defaultCurrency":"BTC"}`, false},
		{ModuleBranding, `{"displayName":"Team","logoUrl":"https://example.com/logo.png","labels":["test"]}`, true},
		{ModuleBranding, `{"displayName":"Team","logoUrl":"http://example.com/logo.png","labels":[]}`, false},
		{ModuleAI, `null`, false},
		{ModuleAI, `[]`, false},
		{"platform", `{}`, false},
	}
	for _, tc := range tests {
		got, err := Validate(tc.module, []byte(tc.raw))
		if (err == nil) != tc.good {
			t.Errorf("%s %s: err=%v", tc.module, tc.raw, err)
		}
		if err == nil && !json.Valid(got) {
			t.Fatal("invalid normalized JSON")
		}
	}
}
func TestCollectSourcePolicy(t *testing.T) {
	if CheckCollectSource(Collect{}, "1688") != nil {
		t.Fatal("empty list must inherit platform choices")
	}
	if CheckCollectSource(Collect{AllowedSources: []string{"1688"}}, "1688") != nil {
		t.Fatal("explicit allowlist")
	}
	if CheckCollectSource(Collect{AllowedSources: []string{"1688"}}, "pinduoduo") == nil {
		t.Fatal("must deny unlisted source")
	}
}

func TestSixLanguageSettingsContract(t *testing.T) {
	for _, language := range []string{"zh-CN", "en-US", "ja-JP", "zh-TW", "ko-KR", "th-TH"} {
		t.Run(language, func(t *testing.T) {
			general, _ := json.Marshal(General{Language: language, Timezone: "UTC", Currency: "USD"})
			if _, err := Validate(ModuleGeneral, general); err != nil {
				t.Fatalf("organization locale rejected: %v", err)
			}
			ai, _ := json.Marshal(AI{DefaultLanguage: language})
			if _, err := Validate(ModuleAI, ai); err != nil {
				t.Fatalf("AI locale rejected: %v", err)
			}
			personal, _ := json.Marshal(PersonalPreferences{Language: language, Theme: "light", DateFormat: "DD/MM/YYYY"})
			if _, err := ValidatePersonalPreferences(personal); err != nil {
				t.Fatalf("personal locale rejected: %v", err)
			}
		})
	}
}

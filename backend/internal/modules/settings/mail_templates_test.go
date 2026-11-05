package settings

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMailTemplatesAreValidForEveryConfiguredEventAndLocale(t *testing.T) {
	for _, event := range mailEvents {
		for _, locale := range mailLocales {
			t.Run(event.Key+"/"+locale, func(t *testing.T) {
				tmpl := defaultMailTemplate(event.Key, locale)
				if err := validateMailTemplate(tmpl); err != nil {
					t.Fatalf("default mail template rejected: %v", err)
				}
				subject, html, err := RenderMailTemplate(tmpl, map[string]string{
					"site_name":         "TradeMind",
					"recipient_email":   "test@example.com",
					"verification_code": "<123456>",
					"event_name":        "Notice",
				})
				if err != nil || subject == "" || html == "" {
					t.Fatalf("render failed: %v", err)
				}
				if strings.Contains(html, "<123456>") {
					t.Fatal("unescaped user-supplied placeholder content")
				}
			})
		}
	}
}

func TestMailTemplateRejectsInjectionAndUnknownVariables(t *testing.T) {
	template := defaultMailTemplate("auth.verify_code", "en-US")
	template.Subject = "Subject\r\nBcc: attacker@example.com"
	if validateMailTemplate(template) == nil {
		t.Fatal("subject CRLF accepted")
	}
	template = defaultMailTemplate("auth.verify_code", "zh-CN")
	template.HTML = "<img src=x onerror=alert(1)>"
	if validateMailTemplate(template) == nil {
		t.Fatal("scriptable event handler accepted")
	}
	template = defaultMailTemplate("auth.verify_code", "zh-CN")
	template.HTML = "{{password}}"
	if validateMailTemplate(template) == nil {
		t.Fatal("unknown placeholder accepted")
	}
}

func TestRegistrationPolicyDefaultsClosedAndRejectsMalformedValues(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:auth_policy_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Setting{}); err != nil {
		t.Fatal(err)
	}
	if got := RegistrationPolicy(context.Background(), db); got != RegistrationClosed {
		t.Fatalf("default policy = %q", got)
	}
	for _, val := range []string{"invalid", "true", ""} {
		row := Setting{TenantID: 0, GroupKey: "auth_registration", ItemKey: "mode", ItemValue: val}
		if err := db.Where("tenant_id = 0 AND group_key = 'auth_registration' AND item_key = 'mode'").Delete(&Setting{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		if got := RegistrationPolicy(context.Background(), db); got != RegistrationClosed {
			t.Fatalf("invalid policy %q unexpectedly returned %q", val, got)
		}
	}
}

func TestLocalizedDefaultTemplatesHaveDistinctEventContent(t *testing.T) {
	for _, locale := range []string{"en-US", "ja-JP", "zh-TW", "ko-KR", "th-TH"} {
		for _, event := range mailEvents {
			template := defaultMailTemplate(event.Key, locale)
			if err := validateMailTemplate(template); err != nil {
				t.Fatalf("%s / %s: %v", locale, event.Key, err)
			}
			title, body := localizedMailCopy(event.Key, locale, "", "")
			if title == "" || body == "" || !strings.Contains(template.Subject, title) ||
				!strings.Contains(template.HTML, body) {
				t.Fatalf("event-specific localization missing: %s / %s", locale, event.Key)
			}
		}
	}
	for _, locale := range []string{"en-US", "ja-JP", "zh-TW", "ko-KR", "th-TH"} {
		codeTitle, _ := localizedMailCopy("auth.verify_code", locale, "", "")
		resetTitle, _ := localizedMailCopy("auth.password_reset", locale, "", "")
		if codeTitle == resetTitle {
			t.Fatalf("verification and reset must not share generic subject: %s", locale)
		}
	}
}

package passwordpolicy

import (
	"strings"
	"testing"
)

func TestValidateNewPassword(t *testing.T) {
	for _, value := range []string{"Test-password-123", "simplepass123", strings.Repeat("a", 71) + "1"} {
		if err := Validate(value); err != nil {
			t.Errorf("valid password rejected: %v", err)
		}
	}
	for name, value := range map[string]string{
		"empty": "", "short": "Pass123!", "too_long": strings.Repeat("a", 72) + "1",
		"chinese": "Password123中文", "full_width": "Password123Ａ", "emoji": "Password123😀",
		"space": "Password 1234", "newline": "Password123\n", "control": "Password123\x7f",
		"no_letter": "123456789012!", "no_digit": "long-password!",
	} {
		t.Run(name, func(t *testing.T) {
			if err := Validate(value); err == nil {
				t.Fatal("invalid password accepted")
			}
		})
	}
}

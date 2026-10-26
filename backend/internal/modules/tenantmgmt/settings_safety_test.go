package tenantmgmt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeTenantSettingsRejectsSecretsAndUnsafeJSON(t *testing.T) {
	for _, raw := range []string{
		`{"password":"not-a-secret"}`,
		`{"apiKey":"example"}`,
		`{"user":{"access_token":"test"}}`,
		`{"nested":[{"clientSecret":"s"}]}`,
		`{"__proto__":{"isRoot":true}}`,
		`{"preferences":"Bearer abcdef"}`,
		`{"preferences":"-----BEGIN PRIVATE KEY-----"}`,
		"[" + strings.Repeat("[]", 33) + "]",
	} {
		_, err := normalizeSettingsJSON(raw)
		require.Error(t, err, raw)
	}
}

func TestNormalizeTenantSettingsKeepsOrdinaryExtensionData(t *testing.T) {
	output, err := normalizeSettingsJSON(`{"locale":"zh-CN","features":{"layout":"compact"},"labels":["a","b"]}`)
	require.NoError(t, err)
	require.JSONEq(t, `{"locale":"zh-CN","features":{"layout":"compact"},"labels":["a","b"]}`, output)
	empty, err := normalizeSettingsJSON("")
	require.NoError(t, err)
	require.Equal(t, "{}", empty)
}

func TestNormalizeTenantSettingsUTF8LimitAndDepth(t *testing.T) {
	_, err := normalizeSettingsJSON(`{"label":"` + strings.Repeat("中", 22000) + `"}`)
	require.Error(t, err)
	value := any("safe")
	for i := 0; i < 33; i++ {
		value = map[string]any{"data": value}
	}
	require.Error(t, rejectSensitiveTenantSettings(value, 0))
}

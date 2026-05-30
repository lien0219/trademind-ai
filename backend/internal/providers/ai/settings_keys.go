package ai

import "strings"

// NormalizeProviderName exports provider name normalization for settings migration.
func NormalizeProviderName(v string) string {
	return normalizeProviderName(v)
}

// ProviderAPIKeyKey returns the settings item key for a provider's API key.
func ProviderAPIKeyKey(provider string) string {
	return normalizeProviderName(provider) + "_api_key"
}

// ProviderBaseURLKey returns the settings item key for a provider's base URL.
func ProviderBaseURLKey(provider string) string {
	return normalizeProviderName(provider) + "_base_url"
}

// ProviderModelKey returns the settings item key for a provider's default model.
func ProviderModelKey(provider string) string {
	return normalizeProviderName(provider) + "_model"
}

// ResolveProviderAPIKey returns the API key for the active provider.
// Falls back to legacy settings.ai.api_key when the provider-specific key is empty.
func ResolveProviderAPIKey(plain map[string]string, provider string) string {
	pname := normalizeProviderName(provider)
	if pname == "" {
		pname = "openai_compatible"
	}
	if key := strings.TrimSpace(plain[ProviderAPIKeyKey(pname)]); key != "" {
		return key
	}
	return strings.TrimSpace(plain["api_key"])
}

// ResolveProviderBaseURL returns the base URL for the active provider.
// Falls back to legacy settings.ai.base_url, then provider defaults.
func ResolveProviderBaseURL(plain map[string]string, provider string) string {
	pname := normalizeProviderName(provider)
	if pname == "" {
		pname = "openai_compatible"
	}
	if base := strings.TrimSpace(plain[ProviderBaseURLKey(pname)]); base != "" {
		return strings.TrimRight(base, "/")
	}
	return resolveBaseURL(pname, "")
}

// ResolveProviderModel returns the model for the active provider.
// Falls back to legacy settings.ai.model, then provider defaults.
func ResolveProviderModel(plain map[string]string, provider, reqModel string) string {
	pname := normalizeProviderName(provider)
	if pname == "" {
		pname = "openai_compatible"
	}
	if m := strings.TrimSpace(reqModel); m != "" {
		return m
	}
	if m := strings.TrimSpace(plain[ProviderModelKey(pname)]); m != "" {
		return m
	}
	return resolveModel(pname, "", "")
}

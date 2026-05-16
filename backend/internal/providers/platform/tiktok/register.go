package tiktok

import platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"

// RegisterProvider registers the TikTok beta provider (must run after BindShops for token refresh persistence).
func RegisterProvider() {
	platformp.Register(NewProvider())
}

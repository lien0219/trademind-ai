package douyinshop

import platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"

// RegisterProvider registers the Douyin Shop Phase 1 provider.
func RegisterProvider() {
	platformp.Register(NewProvider())
}

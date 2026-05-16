package shopee

import platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"

// RegisterProvider registers the Shopee beta OrderSync provider.
func RegisterProvider() {
	platformp.Register(NewProvider())
}

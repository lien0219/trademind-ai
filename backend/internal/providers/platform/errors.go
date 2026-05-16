package platform

import "errors"

// ErrIncompleteTikTokAppConfig is returned when TikTok Partner credentials or redirect_uri are missing in settings.platform_tiktok.
const ErrIncompleteTikTokAppConfig = "platform config incomplete: please configure platform_tiktok.app_key, app_secret and redirect_uri"

// ErrIncompleteShopeeAppConfig is returned when Shopee Partner credentials or redirect_uri are missing in settings.platform_shopee.
const ErrIncompleteShopeeAppConfig = "platform config incomplete: please configure platform_shopee.partner_id, partner_key and redirect_uri"

// ErrIncompleteLazadaAppConfig is returned when Lazada app credentials or redirect_uri are missing in settings.platform_lazada.
const ErrIncompleteLazadaAppConfig = "platform config incomplete: please configure platform_lazada.app_key, app_secret and redirect_uri"

// ErrNotImplemented means the provider is registered but real API/OAuth is not wired yet.
var ErrNotImplemented = errors.New("platform provider not implemented")

// ErrManualOrderSyncUnsupported is returned for manual shops (no remote sync).
var ErrManualOrderSyncUnsupported = errors.New("manual shop does not support order sync")

// ErrOrderSyncNotImplemented is returned when order sync is not wired for a planned provider.
var ErrOrderSyncNotImplemented = errors.New("platform order sync not implemented")

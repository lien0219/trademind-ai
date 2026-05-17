package platform

import "errors"

// ErrIncompleteTikTokAppConfig is returned when TikTok Partner credentials or redirect_uri are missing in settings.platform_tiktok.
const ErrIncompleteTikTokAppConfig = "platform config incomplete: please configure platform_tiktok.app_key, app_secret and redirect_uri"

// ErrIncompleteShopeeAppConfig is returned when Shopee Partner credentials or redirect_uri are missing in settings.platform_shopee.
const ErrIncompleteShopeeAppConfig = "platform config incomplete: please configure platform_shopee.partner_id, partner_key and redirect_uri"

// ErrIncompleteLazadaAppConfig is returned when Lazada app credentials or redirect_uri are missing in settings.platform_lazada.
const ErrIncompleteLazadaAppConfig = "platform config incomplete: please configure platform_lazada.app_key, app_secret and redirect_uri"

// ErrIncompleteAmazonAppConfig is returned when Amazon LWA / SP-API settings are incomplete in settings.platform_amazon.
const ErrIncompleteAmazonAppConfig = "platform config incomplete: please configure platform_amazon.client_id, client_secret, redirect_uri, marketplace_id and sp_api_base_url"

// ErrNotImplemented means the provider is registered but real API/OAuth is not wired yet.
var ErrNotImplemented = errors.New("platform provider not implemented")

// ErrManualOrderSyncUnsupported is returned for manual shops (no remote sync).
var ErrManualOrderSyncUnsupported = errors.New("manual shop does not support order sync")

// ErrOrderSyncNotImplemented is returned when order sync is not wired for a planned provider.
var ErrOrderSyncNotImplemented = errors.New("platform order sync not implemented")

// ErrManualCustomerMessageUnsupported is returned for manual shops (no remote messaging).
var ErrManualCustomerMessageUnsupported = errors.New("manual shop does not support platform customer messages")

// ErrCustomerMessageNotImplemented is returned when provider has no live buyer-seller messaging API wired yet.
var ErrCustomerMessageNotImplemented = errors.New("platform customer message provider not implemented")

// ErrManualProductPublishUnsupported is returned for manual shops (no listings API).
var ErrManualProductPublishUnsupported = errors.New("manual shop does not support product publish")

// ErrProductPublishNotImplemented means real listings API / worker path is unavailable for this platform build.
var ErrProductPublishNotImplemented = errors.New("platform product publish provider not implemented")

// ErrPlatformCustomerMessagePermissionDenied is returned when token/scopes or platform account forbids chat APIs.
var ErrPlatformCustomerMessagePermissionDenied = errors.New("platform customer message permission denied or not configured")

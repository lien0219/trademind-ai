package platform

import "errors"

// ErrNotImplemented means the provider is registered but real API/OAuth is not wired yet.
var ErrNotImplemented = errors.New("platform provider not implemented")

// ErrManualOrderSyncUnsupported is returned for manual shops (no remote sync).
var ErrManualOrderSyncUnsupported = errors.New("manual shop does not support order sync")

// ErrOrderSyncNotImplemented is returned when order sync is not wired for a planned provider.
var ErrOrderSyncNotImplemented = errors.New("platform order sync not implemented")

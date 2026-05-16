package platform

import "errors"

// ErrNotImplemented means the provider is registered but real API/OAuth is not wired yet.
var ErrNotImplemented = errors.New("platform provider not implemented")

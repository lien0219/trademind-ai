package shop

// Shop operational status
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// Auth lifecycle
const (
	AuthUnauthorized = "unauthorized"
	AuthAuthorized   = "authorized"
	AuthExpired      = "expired"
	AuthError        = "error"
	AuthUnsupported  = "unsupported"
)

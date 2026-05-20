package response

// Business-layer result codes (HTTP status may differ).
const (
	CodeOK                            = 0
	CodeBadRequest                    = 40001
	CodeCustomCollectProviderConflict = 40002
	CodeUnauthorized                  = 40101
	CodeForbidden                     = 40301
	CodeNotFound                      = 40401
	CodeInternalError                 = 50000
	// CodeServiceUnavailable indicates dependency unavailable (e.g. Redis queue).
	CodeServiceUnavailable = 50301
)

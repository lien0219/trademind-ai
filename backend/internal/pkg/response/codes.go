package response

// Business-layer result codes (HTTP status may differ).
const (
	CodeOK                            = 0
	CodeBadRequest                    = 40001
	CodeCustomCollectProviderConflict = 40002
	CodeAIRuleInvalid                 = 40003
	CodePublishConfigInvalid          = 40004
	CodeUnauthorized                  = 40101
	CodeForbidden                     = 40301
	CodePermissionDenied              = 40302
	CodeStorePermissionDenied         = 40303
	CodeReadonlyForbidden             = 40304
	CodeSettingsPermissionRequired    = 40305
	CodeUserManagePermissionRequired  = 40306
	CodeNotFound                      = 40401
	CodeInternalError                 = 50000
	// CodeServiceUnavailable indicates dependency unavailable (e.g. Redis queue).
	CodeServiceUnavailable = 50301
)

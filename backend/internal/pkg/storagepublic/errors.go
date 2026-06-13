package storagepublic

// Error codes for storage public access verification.
const (
	CodePublicBaseMissing        = "STORAGE_PUBLIC_BASE_MISSING"
	CodePublicURLInvalid         = "STORAGE_PUBLIC_URL_INVALID"
	CodePublicURLPrivate         = "STORAGE_PUBLIC_URL_PRIVATE"
	CodePublicAccessFailed       = "STORAGE_PUBLIC_ACCESS_FAILED"
	CodePublicRedirected         = "STORAGE_PUBLIC_REDIRECTED"
	CodePublicContentTypeInvalid = "STORAGE_PUBLIC_CONTENT_TYPE_INVALID"
	CodePublicImageDecodeFailed  = "STORAGE_PUBLIC_IMAGE_DECODE_FAILED"
	CodePublicCertificateInvalid = "STORAGE_PUBLIC_CERTIFICATE_INVALID"
)

// VerifyError carries a stable error code for API responses.
type VerifyError struct {
	Code    string
	Message string
	Details map[string]any
}

func (e *VerifyError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func verifyErr(code, msg string, details map[string]any) *VerifyError {
	return &VerifyError{Code: code, Message: msg, Details: details}
}

package response

// Envelope is the unified JSON shape for all API responses.
type Envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	TraceID string      `json:"traceId,omitempty"`
}

package ai

// Message is one chat message for the provider.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseFormat enables JSON mode on compatible APIs (e.g. OpenAI response_format).
type ResponseFormat struct {
	Type string `json:"type"` // e.g. "json_object"
}

// ChatRequest is a normalized chat completion request.
type ChatRequest struct {
	Model          string
	Messages       []Message
	Temperature    float64
	MaxTokens      int
	ResponseFormat *ResponseFormat
}

// ChatResponse is a normalized chat completion response.
type ChatResponse struct {
	Content      string
	Raw          []byte
	Model        string
	InputTokens  int
	OutputTokens int
}

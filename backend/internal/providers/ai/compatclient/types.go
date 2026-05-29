package compatclient

// Message is one chat message for Chat Completions.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ContentPart is one multimodal message part (OpenAI-compatible).
type ContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *ImageURLDetail `json:"image_url,omitempty"`
}

// ImageURLDetail wraps an image URL for vision models.
type ImageURLDetail struct {
	URL string `json:"url"`
}

// Request is a chat/completions payload.
type Request struct {
	Model           string
	Messages        []Message
	Temperature     float64
	MaxTokens       int
	ResponseFormat  string // e.g. json_object
	DisableThinking bool   // DeepSeek v4: disable thinking for structured JSON output
}

// Result is a normalized completion outcome.
type Result struct {
	Content      string
	Raw          []byte
	Model        string
	InputTokens  int
	OutputTokens int
}

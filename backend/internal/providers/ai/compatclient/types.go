package compatclient

// Message is one chat message for Chat Completions.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

package customerchat

// Conversation statuses
const (
	StatusOpen         = "open"
	StatusPendingReply = "pending_reply"
	StatusReplied      = "replied"
	StatusClosed       = "closed"
)

// Message roles
const (
	RoleCustomer = "customer"
	RoleAgent    = "agent"
	RoleAI       = "ai"
)

// Message sources
const (
	SourceManual       = "manual"
	SourceImported     = "imported"
	SourcePlatform     = "platform"
	SourceAISuggestion = "ai_suggestion"
)

// Suggestion statuses
const (
	SuggestionGenerated = "generated"
	SuggestionEdited    = "edited"
	SuggestionAccepted  = "accepted"
	SuggestionDiscarded = "discarded"
)

// TaskTypeCustomerReplyGenerate is recorded on ai_tasks.task_type
const TaskTypeCustomerReplyGenerate = "customer_reply_generate"

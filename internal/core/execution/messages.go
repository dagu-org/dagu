package execution

// LLMMessages stores conversation messages for all LLM steps in a DAG run.
type LLMMessages struct {
	// Steps maps step names to their conversation messages.
	Steps map[string][]LLMMessage `json:"steps"`
}

// LLMMessage represents a single message in the conversation.
type LLMMessage struct {
	// Role is the message role (system, user, assistant).
	Role string `json:"role"`
	// Content is the message content.
	Content string `json:"content"`
	// Metadata contains API call metadata (only set for assistant responses).
	Metadata *LLMMessageMetadata `json:"metadata,omitempty"`
}

// LLMMessageMetadata contains metadata about an LLM API call.
type LLMMessageMetadata struct {
	// Provider is the LLM provider used (openai, anthropic, etc.).
	Provider string `json:"provider,omitempty"`
	// Model is the model identifier used.
	Model string `json:"model,omitempty"`
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int `json:"promptTokens,omitempty"`
	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int `json:"completionTokens,omitempty"`
	// TotalTokens is the sum of prompt and completion tokens.
	TotalTokens int `json:"totalTokens,omitempty"`
}

// NewLLMMessages creates a new empty LLMMessages.
func NewLLMMessages() *LLMMessages {
	return &LLMMessages{
		Steps: make(map[string][]LLMMessage),
	}
}

// GetStepMessages returns the messages for a specific step.
// Returns nil if the step has no messages.
func (m *LLMMessages) GetStepMessages(stepName string) []LLMMessage {
	if m == nil || m.Steps == nil {
		return nil
	}
	return m.Steps[stepName]
}

// SetStepMessages sets the messages for a specific step.
func (m *LLMMessages) SetStepMessages(stepName string, messages []LLMMessage) {
	if m.Steps == nil {
		m.Steps = make(map[string][]LLMMessage)
	}
	m.Steps[stepName] = messages
}

// MergeFromDependencies collects and merges messages from dependent steps.
// Returns deduplicated messages with only the first system message kept.
func (m *LLMMessages) MergeFromDependencies(depends []string) []LLMMessage {
	if m == nil || m.Steps == nil || len(depends) == 0 {
		return nil
	}

	var merged []LLMMessage
	for _, dep := range depends {
		if msgs := m.Steps[dep]; len(msgs) > 0 {
			merged = append(merged, msgs...)
		}
	}

	return DeduplicateSystemMessages(merged)
}

// DeduplicateSystemMessages keeps only the first system message.
func DeduplicateSystemMessages(messages []LLMMessage) []LLMMessage {
	if len(messages) == 0 {
		return nil
	}

	result := make([]LLMMessage, 0, len(messages))
	seenSystem := false

	for _, msg := range messages {
		if msg.Role == "system" {
			if seenSystem {
				continue
			}
			seenSystem = true
		}
		result = append(result, msg)
	}

	return result
}

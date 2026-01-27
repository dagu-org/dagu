// Package agent provides an AI-powered chat interface for managing DAGs.
package agent

import (
	"encoding/json"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

// MessageType identifies the type of message in a conversation.
type MessageType string

const (
	// MessageTypeUser is a message from the user.
	MessageTypeUser MessageType = "user"
	// MessageTypeAssistant is a message from the AI assistant.
	MessageTypeAssistant MessageType = "assistant"
	// MessageTypeSystem is a system message (e.g., system prompt).
	MessageTypeSystem MessageType = "system"
	// MessageTypeError is an error message.
	MessageTypeError MessageType = "error"
)

// Message represents a message in a conversation.
// This is the format stored and sent to the UI.
type Message struct {
	ID             string           `json:"id"`
	ConversationID string           `json:"conversation_id"`
	Type           MessageType      `json:"type"`
	SequenceID     int64            `json:"sequence_id"`
	Content        string           `json:"content,omitempty"`
	ToolCalls      []llm.ToolCall   `json:"tool_calls,omitempty"`
	ToolResults    []ToolResult     `json:"tool_results,omitempty"`
	Usage          *llm.Usage       `json:"usage,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	LLMData        *llm.Message     `json:"llm_data,omitempty"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// Conversation represents a chat conversation.
type Conversation struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationState represents the current state of a conversation.
type ConversationState struct {
	ConversationID string `json:"conversation_id"`
	Working        bool   `json:"working"`
	Model          string `json:"model,omitempty"`
}

// StreamResponse is sent over SSE to the UI.
type StreamResponse struct {
	Messages          []Message          `json:"messages,omitempty"`
	Conversation      *Conversation      `json:"conversation,omitempty"`
	ConversationState *ConversationState `json:"conversation_state,omitempty"`
}

// ChatRequest is the request body for sending a chat message.
type ChatRequest struct {
	Message string `json:"message"`
	Model   string `json:"model,omitempty"`
}

// NewConversationResponse is the response for creating a new conversation.
type NewConversationResponse struct {
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
}

// ToolOut represents the output of a tool execution.
// This follows the Shelley pattern where tools return structured output.
type ToolOut struct {
	// Content is the output to be sent back to the LLM.
	Content string
	// IsError indicates if the tool execution failed.
	IsError bool
	// Display is optional content for the UI (e.g., rendered output).
	Display any
}

// ToolFunc is the function signature for tool execution.
type ToolFunc func(ctx ToolContext, input json.RawMessage) ToolOut

// ToolContext provides context to tool execution.
type ToolContext struct {
	WorkingDir string
}

// AgentTool extends llm.Tool with an execution function.
type AgentTool struct {
	llm.Tool
	// Run executes the tool with the given input.
	Run ToolFunc
}

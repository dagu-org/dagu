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
	// MessageTypeUIAction is a UI action message (e.g., navigate to page).
	MessageTypeUIAction MessageType = "ui_action"
)

// UIAction represents an action to be performed by the UI.
type UIAction struct {
	// Type specifies the action kind (e.g., "navigate", "refresh").
	Type string `json:"type"`
	// Path is the navigation target for "navigate" actions.
	Path string `json:"path,omitempty"`
}

// Message represents a message in a conversation, stored and sent to the UI.
type Message struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	Type           MessageType    `json:"type"`
	SequenceID     int64          `json:"sequence_id"`
	Content        string         `json:"content,omitempty"`
	ToolCalls      []llm.ToolCall `json:"tool_calls,omitempty"`
	ToolResults    []ToolResult   `json:"tool_results,omitempty"`
	Usage          *llm.Usage     `json:"usage,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	LLMData        *llm.Message   `json:"llm_data,omitempty"`
	UIAction       *UIAction      `json:"ui_action,omitempty"`
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
	UserID    string    `json:"user_id,omitempty"`
	Title     string    `json:"title,omitempty"`
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

// DAGContext contains a single DAG reference from the frontend.
type DAGContext struct {
	DAGFile  string `json:"dag_file"`
	DAGRunID string `json:"dag_run_id,omitempty"`
}

// ChatRequest is the request body for sending a chat message.
type ChatRequest struct {
	Message     string       `json:"message"`
	Model       string       `json:"model,omitempty"`
	DAGContexts []DAGContext `json:"dag_contexts,omitempty"`
}

// ResolvedDAGContext contains server-resolved info for a single DAG.
type ResolvedDAGContext struct {
	// DAGFilePath is the absolute path to the DAG file.
	DAGFilePath string
	// DAGName is the name of the DAG.
	DAGName string
	// DAGRunID is present when viewing a specific run.
	DAGRunID string
	// RunStatus indicates the run state (running, success, or failed).
	RunStatus string
}

// NewConversationResponse is the response for creating a new conversation.
type NewConversationResponse struct {
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
}

// ToolOut represents the output of a tool execution.
type ToolOut struct {
	// Content is the output sent back to the LLM.
	Content string
	// IsError indicates tool execution failure.
	IsError bool
	// Display is optional UI content (e.g., rendered output).
	Display any
}

// ToolFunc is the function signature for tool execution.
type ToolFunc func(ctx ToolContext, input json.RawMessage) ToolOut

// UIActionFunc is the function signature for emitting UI actions.
type UIActionFunc func(action UIAction)

// ToolContext provides context to tool execution.
type ToolContext struct {
	WorkingDir   string
	EmitUIAction UIActionFunc
}

// AgentTool extends llm.Tool with an execution function.
type AgentTool struct {
	llm.Tool
	Run ToolFunc
}

// EnvironmentInfo contains Dagu environment paths for the system prompt.
type EnvironmentInfo struct {
	DAGsDir    string
	LogDir     string
	DataDir    string
	ConfigFile string
}

// Package agent provides an AI-powered chat interface for managing DAGs.
package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

// MessageType identifies the type of message in a conversation.
type MessageType string

const (
	// MessageTypeUser represents a message from the user.
	MessageTypeUser MessageType = "user"
	// MessageTypeAssistant represents a message from the AI assistant.
	MessageTypeAssistant MessageType = "assistant"
	// MessageTypeError represents an error message.
	MessageTypeError MessageType = "error"
	// MessageTypeUIAction represents a UI action to be performed.
	MessageTypeUIAction MessageType = "ui_action"
)

// UIAction represents an action to be performed by the UI (e.g., navigate).
type UIAction struct {
	// Type is the action type (e.g., "navigate").
	Type string `json:"type"`
	// Path is the target path for navigation actions.
	Path string `json:"path,omitempty"`
}

// Message represents a message in a conversation.
type Message struct {
	// ID is the unique identifier for this message.
	ID string `json:"id"`
	// ConversationID links this message to its parent conversation.
	ConversationID string `json:"conversation_id"`
	// Type identifies the message type (user, assistant, error, ui_action).
	Type MessageType `json:"type"`
	// SequenceID orders messages within a conversation.
	SequenceID int64 `json:"sequence_id"`
	// Content is the text content of the message.
	Content string `json:"content,omitempty"`
	// ToolCalls contains tool calls made by the assistant.
	ToolCalls []llm.ToolCall `json:"tool_calls,omitempty"`
	// ToolResults contains results from executed tool calls.
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	// Usage contains token usage statistics.
	Usage *llm.Usage `json:"usage,omitempty"`
	// CreatedAt is when this message was created.
	CreatedAt time.Time `json:"created_at"`
	// LLMData contains the original LLM message for provider reconstruction.
	LLMData *llm.Message `json:"llm_data,omitempty"`
	// UIAction contains a UI action when Type is MessageTypeUIAction.
	UIAction *UIAction `json:"ui_action,omitempty"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	// ToolCallID links this result to its corresponding tool call.
	ToolCallID string `json:"tool_call_id"`
	// Content is the output from the tool execution.
	Content string `json:"content"`
	// IsError indicates whether the tool execution failed.
	IsError bool `json:"is_error,omitempty"`
}

// Conversation represents a chat conversation with metadata.
type Conversation struct {
	// ID is the unique identifier for this conversation.
	ID string `json:"id"`
	// UserID identifies the user who owns this conversation.
	UserID string `json:"user_id,omitempty"`
	// Title is a human-readable name for the conversation.
	Title string `json:"title,omitempty"`
	// CreatedAt is when this conversation was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when this conversation was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationState represents the current state of a conversation.
type ConversationState struct {
	// ConversationID identifies which conversation this state belongs to.
	ConversationID string `json:"conversation_id"`
	// Working indicates whether the agent is currently processing.
	Working bool `json:"working"`
	// Model is the LLM model being used for this conversation.
	Model string `json:"model,omitempty"`
}

// StreamResponse is sent over SSE to the UI.
type StreamResponse struct {
	// Messages contains new or updated messages.
	Messages []Message `json:"messages,omitempty"`
	// Conversation contains conversation metadata updates.
	Conversation *Conversation `json:"conversation,omitempty"`
	// ConversationState contains the current processing state.
	ConversationState *ConversationState `json:"conversation_state,omitempty"`
}

// DAGContext contains a DAG reference from the frontend.
type DAGContext struct {
	// DAGFile is the DAG file path or identifier.
	DAGFile string `json:"dag_file"`
	// DAGRunID identifies a specific run of the DAG.
	DAGRunID string `json:"dag_run_id,omitempty"`
}

// ChatRequest is the request body for sending a chat message.
type ChatRequest struct {
	// Message is the user's input text.
	Message string `json:"message"`
	// Model specifies which LLM model to use.
	Model string `json:"model,omitempty"`
	// DAGContexts provides DAG references for context-aware responses.
	DAGContexts []DAGContext `json:"dag_contexts,omitempty"`
}

// ResolvedDAGContext contains server-resolved information for a DAG.
type ResolvedDAGContext struct {
	// DAGFilePath is the absolute path to the DAG file.
	DAGFilePath string
	// DAGName is the human-readable name of the DAG.
	DAGName string
	// DAGRunID identifies a specific run (present when viewing a specific run).
	DAGRunID string
	// RunStatus is the execution status (running, success, or failed).
	RunStatus string
}

// NewConversationResponse is the response for creating a new conversation.
type NewConversationResponse struct {
	// ConversationID is the ID of the newly created conversation.
	ConversationID string `json:"conversation_id"`
	// Status indicates the result of the creation request.
	Status string `json:"status"`
}

// ToolOut represents the output of a tool execution.
type ToolOut struct {
	// Content is the output sent back to the LLM.
	Content string
	// IsError indicates whether the tool execution failed.
	IsError bool
}

// ToolFunc is the function signature for tool execution.
type ToolFunc func(ctx ToolContext, input json.RawMessage) ToolOut

// UIActionFunc emits UI actions during tool execution.
type UIActionFunc func(action UIAction)

// ToolContext provides context for tool execution.
type ToolContext struct {
	// Context is the parent context for cancellation propagation.
	Context context.Context
	// WorkingDir is the current working directory for the tool.
	WorkingDir string
	// EmitUIAction is a callback to emit UI actions during execution.
	EmitUIAction UIActionFunc
}

// AgentTool extends llm.Tool with an execution function.
type AgentTool struct {
	llm.Tool
	// Run is the function that executes this tool.
	Run ToolFunc
}

// EnvironmentInfo contains Dagu environment paths for the system prompt.
type EnvironmentInfo struct {
	// DAGsDir is the directory containing DAG definition files.
	DAGsDir string
	// LogDir is the directory for log files.
	LogDir string
	// DataDir is the directory for data storage.
	DataDir string
	// ConfigFile is the path to the configuration file.
	ConfigFile string
	// WorkingDir is the current working directory.
	WorkingDir string
}

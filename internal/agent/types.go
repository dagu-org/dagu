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
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeError     MessageType = "error"
	MessageTypeUIAction  MessageType = "ui_action"
)

// UIAction represents an action to be performed by the UI (e.g., navigate).
type UIAction struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

// Message represents a message in a conversation.
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

// Conversation represents a chat conversation with metadata.
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

// DAGContext contains a DAG reference from the frontend.
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

// ResolvedDAGContext contains server-resolved information for a DAG.
type ResolvedDAGContext struct {
	DAGFilePath string // Absolute path to the DAG file
	DAGName     string
	DAGRunID    string // Present when viewing a specific run
	RunStatus   string // Running, success, or failed
}

// NewConversationResponse is the response for creating a new conversation.
type NewConversationResponse struct {
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
}

// ToolOut represents the output of a tool execution.
type ToolOut struct {
	Content string // Output sent back to the LLM
	IsError bool
}

// ToolFunc is the function signature for tool execution.
type ToolFunc func(ctx ToolContext, input json.RawMessage) ToolOut

// UIActionFunc emits UI actions during tool execution.
type UIActionFunc func(action UIAction)

// ToolContext provides context for tool execution.
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
	WorkingDir string
}

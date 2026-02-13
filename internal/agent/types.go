// Package agent provides an AI-powered chat interface for managing DAGs.
package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

// MessageType identifies the type of message in a session.
type MessageType string

// PromptType identifies the type of user prompt.
type PromptType string

const (
	// PromptTypeGeneral represents a general question prompt.
	PromptTypeGeneral PromptType = "general"
	// PromptTypeCommandApproval represents a command approval prompt.
	PromptTypeCommandApproval PromptType = "command_approval"
)

const (
	// MessageTypeUser represents a message from the user.
	MessageTypeUser MessageType = "user"
	// MessageTypeAssistant represents a message from the AI assistant.
	MessageTypeAssistant MessageType = "assistant"
	// MessageTypeError represents an error message.
	MessageTypeError MessageType = "error"
	// MessageTypeUIAction represents a UI action to be performed.
	MessageTypeUIAction MessageType = "ui_action"
	// MessageTypeUserPrompt represents a question from agent to user.
	MessageTypeUserPrompt MessageType = "user_prompt"
)

// UIAction represents an action to be performed by the UI (e.g., navigate).
type UIAction struct {
	// Type is the action type (e.g., "navigate").
	Type string `json:"type"`
	// Path is the target path for navigation actions.
	Path string `json:"path,omitempty"`
}

// UserPromptOption represents a single option for a user prompt.
type UserPromptOption struct {
	// ID is the unique identifier for this option.
	ID string `json:"id"`
	// Label is the display text for this option.
	Label string `json:"label"`
	// Description provides additional context for this option.
	Description string `json:"description,omitempty"`
}

// UserPrompt represents a question from agent to user.
type UserPrompt struct {
	// PromptID is the unique identifier for this prompt.
	PromptID string `json:"prompt_id"`
	// Question is the question text to display.
	Question string `json:"question"`
	// Options are the predefined choices (2-4 options).
	Options []UserPromptOption `json:"options,omitempty"`
	// AllowFreeText enables an optional text input field.
	AllowFreeText bool `json:"allow_free_text"`
	// FreeTextPlaceholder is the placeholder for the text input.
	FreeTextPlaceholder string `json:"free_text_placeholder,omitempty"`
	// MultiSelect allows selecting multiple options.
	MultiSelect bool `json:"multi_select"`
	// PromptType identifies the type of prompt (general, command_approval).
	PromptType PromptType `json:"prompt_type,omitempty"`
	// Command is the shell command requiring approval (for command_approval type).
	Command string `json:"command,omitempty"`
	// WorkingDir is the working directory for the command (for command_approval type).
	WorkingDir string `json:"working_dir,omitempty"`
}

// UserPromptResponse is the user's answer to a prompt.
type UserPromptResponse struct {
	// PromptID identifies which prompt this responds to.
	PromptID string `json:"prompt_id"`
	// SelectedOptionIDs are the IDs of selected options.
	SelectedOptionIDs []string `json:"selected_option_ids,omitempty"`
	// FreeTextResponse is the user's text input.
	FreeTextResponse string `json:"free_text_response,omitempty"`
	// Cancelled indicates the user skipped the prompt.
	Cancelled bool `json:"cancelled,omitempty"`
}

// Message represents a message in a session.
type Message struct {
	// ID is the unique identifier for this message.
	ID string `json:"id"`
	// SessionID links this message to its parent session.
	SessionID string `json:"session_id"`
	// Type identifies the message type (user, assistant, error, ui_action, user_prompt).
	Type MessageType `json:"type"`
	// SequenceID orders messages within a session.
	SequenceID int64 `json:"sequence_id"`
	// Content is the text content of the message.
	Content string `json:"content,omitempty"`
	// ToolCalls contains tool calls made by the assistant.
	ToolCalls []llm.ToolCall `json:"tool_calls,omitempty"`
	// ToolResults contains results from executed tool calls.
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	// Usage contains token usage statistics.
	Usage *llm.Usage `json:"usage,omitempty"`
	// Cost is the estimated cost of this message in USD.
	Cost *float64 `json:"cost,omitempty"`
	// CreatedAt is when this message was created.
	CreatedAt time.Time `json:"created_at"`
	// LLMData contains the original LLM message for provider reconstruction.
	LLMData *llm.Message `json:"llm_data,omitempty"`
	// UIAction contains a UI action when Type is MessageTypeUIAction.
	UIAction *UIAction `json:"ui_action,omitempty"`
	// UserPrompt contains a prompt when Type is MessageTypeUserPrompt.
	UserPrompt *UserPrompt `json:"user_prompt,omitempty"`
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

// Session represents a chat session with metadata.
type Session struct {
	// ID is the unique identifier for this session.
	ID string `json:"id"`
	// UserID identifies the user who owns this session.
	UserID string `json:"user_id,omitempty"`
	// DAGName stores the primary DAG context for this session's memory scope.
	DAGName string `json:"dag_name,omitempty"`
	// Title is a human-readable name for the session.
	Title string `json:"title,omitempty"`
	// CreatedAt is when this session was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when this session was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionState represents the current state of a session.
type SessionState struct {
	// SessionID identifies which session this state belongs to.
	SessionID string `json:"session_id"`
	// Working indicates whether the agent is currently processing.
	Working bool `json:"working"`
	// Model is the LLM model being used for this session.
	Model string `json:"model,omitempty"`
	// TotalCost is the accumulated cost of the session in USD.
	TotalCost float64 `json:"total_cost"`
}

// StreamResponse is sent over SSE to the UI.
type StreamResponse struct {
	// Messages contains new or updated messages.
	Messages []Message `json:"messages,omitempty"`
	// Session contains session metadata updates.
	Session *Session `json:"session,omitempty"`
	// SessionState contains the current processing state.
	SessionState *SessionState `json:"session_state,omitempty"`
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
	// SafeMode enables approval prompts for dangerous commands when true.
	SafeMode bool `json:"safe_mode,omitempty"`
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

// NewSessionResponse is the response for creating a new session.
type NewSessionResponse struct {
	// SessionID is the ID of the newly created session.
	SessionID string `json:"session_id"`
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

// EmitUserPromptFunc emits a user prompt during tool execution.
type EmitUserPromptFunc func(prompt UserPrompt)

// WaitUserResponseFunc blocks until user responds to a prompt.
type WaitUserResponseFunc func(ctx context.Context, promptID string) (UserPromptResponse, error)

// ToolContext provides context for tool execution.
type ToolContext struct {
	// Context is the parent context for cancellation propagation.
	Context context.Context
	// WorkingDir is the current working directory for the tool.
	WorkingDir string
	// EmitUIAction is a callback to emit UI actions during execution.
	EmitUIAction UIActionFunc
	// EmitUserPrompt is a callback to emit user prompts during execution.
	EmitUserPrompt EmitUserPromptFunc
	// WaitUserResponse blocks until user responds to a prompt.
	WaitUserResponse WaitUserResponseFunc
	// SafeMode enables approval prompts for dangerous commands when true.
	SafeMode bool
}

// AuditInfo configures how a tool's executions appear in audit logs.
// Nil means the tool is not audited.
type AuditInfo struct {
	// Action is the audit action name (e.g. "bash_exec", "file_read").
	Action string
	// DetailExtractor extracts audit details from the tool's input JSON.
	// If nil, only the tool name is logged.
	DetailExtractor func(input json.RawMessage) map[string]any
}

// ExtractFields returns a DetailExtractor that pulls the named fields from
// the tool's JSON input. Only non-nil values are included in the result.
func ExtractFields(fields ...string) func(json.RawMessage) map[string]any {
	return func(input json.RawMessage) map[string]any {
		var raw map[string]any
		_ = json.Unmarshal(input, &raw)
		result := make(map[string]any, len(fields))
		for _, f := range fields {
			if v, ok := raw[f]; ok {
				result[f] = v
			}
		}
		return result
	}
}

// AgentTool extends llm.Tool with an execution function.
type AgentTool struct {
	llm.Tool
	// Run is the function that executes this tool.
	Run ToolFunc
	// Audit configures audit logging for this tool. Nil means not audited.
	Audit *AuditInfo
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
	// BaseConfigFile is the path to the base configuration file.
	BaseConfigFile string
}

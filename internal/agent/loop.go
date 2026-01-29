package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

// MessageRecordFunc is called to record new messages to persistent storage.
type MessageRecordFunc func(ctx context.Context, message Message) error

// LoopConfig contains configuration for creating a Loop.
type LoopConfig struct {
	// Provider is the LLM provider for making requests.
	Provider llm.Provider
	// Model is the model ID to use for requests.
	Model string
	// History is the initial conversation history.
	History []llm.Message
	// Tools is the list of tools available to the agent.
	Tools []*AgentTool
	// RecordMessage is called to record new messages.
	RecordMessage MessageRecordFunc
	// Logger for logging events.
	Logger *slog.Logger
	// SystemPrompt is the system message to prepend.
	SystemPrompt string
	// WorkingDir is the working directory for tools.
	WorkingDir string
	// ConversationID is the ID of the conversation.
	ConversationID string
	// OnWorking is called when the working state changes.
	OnWorking func(working bool)
	// EmitUIAction is called when a tool wants to emit a UI action.
	EmitUIAction UIActionFunc
}

// Loop manages a conversation turn with an LLM including tool execution.
type Loop struct {
	provider       llm.Provider
	model          string
	tools          []*AgentTool
	recordMessage  MessageRecordFunc
	history        []llm.Message
	messageQueue   []llm.Message
	totalUsage     llm.Usage
	mu             sync.Mutex
	logger         *slog.Logger
	systemPrompt   string
	workingDir     string
	conversationID string
	onWorking      func(working bool)
	sequenceID     int64
	emitUIAction   UIActionFunc
}

// NewLoop creates a new Loop instance.
func NewLoop(config LoopConfig) *Loop {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Loop{
		provider:       config.Provider,
		model:          config.Model,
		history:        config.History,
		tools:          config.Tools,
		recordMessage:  config.RecordMessage,
		messageQueue:   make([]llm.Message, 0),
		logger:         logger,
		systemPrompt:   config.SystemPrompt,
		workingDir:     config.WorkingDir,
		conversationID: config.ConversationID,
		onWorking:      config.OnWorking,
		emitUIAction:   config.EmitUIAction,
	}
}

// QueueUserMessage adds a user message to the queue to be processed.
func (l *Loop) QueueUserMessage(message llm.Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messageQueue = append(l.messageQueue, message)
	l.logger.Debug("queued user message", "content_length", len(message.Content))
}

// Go runs the conversation loop until the context is canceled.
func (l *Loop) Go(ctx context.Context) error {
	if l.provider == nil {
		return fmt.Errorf("no LLM provider configured")
	}

	l.logger.Info("starting conversation loop", "tools", len(l.tools))

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("conversation loop canceled")
			return ctx.Err()
		default:
		}

		// Process any queued messages
		l.mu.Lock()
		hasQueuedMessages := len(l.messageQueue) > 0
		if hasQueuedMessages {
			l.history = append(l.history, l.messageQueue...)
			l.messageQueue = l.messageQueue[:0]
		}
		l.mu.Unlock()

		if hasQueuedMessages {
			l.logger.Debug("processing queued messages")
			if err := l.processLLMRequest(ctx); err != nil {
				l.logger.Error("failed to process LLM request", "error", err)
				time.Sleep(time.Second)
				continue
			}
			l.logger.Debug("finished processing queued messages")
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

// processLLMRequest sends a request to the LLM and handles the response.
func (l *Loop) processLLMRequest(ctx context.Context) error {
	l.mu.Lock()
	history := append([]llm.Message(nil), l.history...)
	l.mu.Unlock()

	llmMessages := l.buildMessages(history)
	llmTools := l.buildToolDefinitions()

	req := &llm.ChatRequest{
		Model:    l.model,
		Messages: llmMessages,
		Tools:    llmTools,
	}

	l.logger.Debug("sending LLM request",
		"message_count", len(llmMessages),
		"tool_count", len(llmTools),
		"model", l.model)

	// Set agent as working
	l.setWorking(true)

	// Add a timeout for the LLM request
	llmCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	resp, err := l.provider.Chat(llmCtx, req)
	if err != nil {
		// Record the error as a message
		l.recordErrorMessage(ctx, fmt.Sprintf("LLM request failed: %v", err))
		l.setWorking(false)
		return fmt.Errorf("LLM request failed: %w", err)
	}

	l.logger.Debug("received LLM response",
		"content_length", len(resp.Content),
		"finish_reason", resp.FinishReason,
		"tool_calls", len(resp.ToolCalls))

	l.accumulateUsage(resp.Usage)
	l.recordAssistantMessage(ctx, resp)

	if len(resp.ToolCalls) > 0 {
		l.logger.Debug("handling tool calls", "count", len(resp.ToolCalls))
		return l.handleToolCalls(ctx, resp.ToolCalls)
	}

	l.setWorking(false)
	return nil
}

// setWorking safely calls the onWorking callback if configured.
func (l *Loop) setWorking(working bool) {
	if l.onWorking != nil {
		l.onWorking(working)
	}
}

// appendToHistory adds a message to history and returns the new sequence ID.
func (l *Loop) appendToHistory(msg llm.Message) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.history = append(l.history, msg)
	l.sequenceID++
	return l.sequenceID
}

// executeTool runs a single tool call and returns the result.
func (l *Loop) executeTool(tc llm.ToolCall) ToolOut {
	tool := GetToolByName(l.tools, tc.Function.Name)
	if tool == nil {
		l.logger.Error("tool not found", "name", tc.Function.Name)
		return ToolOut{
			Content: fmt.Sprintf("Tool '%s' not found", tc.Function.Name),
			IsError: true,
		}
	}

	input := json.RawMessage(tc.Function.Arguments)
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}

	return tool.Run(ToolContext{
		WorkingDir:   l.workingDir,
		EmitUIAction: l.emitUIAction,
	}, input)
}

// handleToolCalls processes tool calls from the LLM response.
func (l *Loop) handleToolCalls(ctx context.Context, toolCalls []llm.ToolCall) error {
	for _, tc := range toolCalls {
		l.logger.Debug("executing tool", "name", tc.Function.Name, "id", tc.ID)
		l.recordToolResult(ctx, tc, l.executeTool(tc))
	}
	return l.processLLMRequest(ctx)
}

// recordToolResult adds a tool result to history and records it.
func (l *Loop) recordToolResult(ctx context.Context, tc llm.ToolCall, result ToolOut) {
	toolMessage := llm.Message{
		Role:       llm.RoleTool,
		Content:    result.Content,
		ToolCallID: tc.ID,
	}
	seqID := l.appendToHistory(toolMessage)

	if l.recordMessage == nil {
		return
	}

	msg := Message{
		ConversationID: l.conversationID,
		Type:           MessageTypeUser, // Tool results are from user perspective
		SequenceID:     seqID,
		ToolResults: []ToolResult{{
			ToolCallID: tc.ID,
			Content:    result.Content,
			IsError:    result.IsError,
		}},
		CreatedAt: time.Now(),
		LLMData:   &toolMessage,
	}
	if err := l.recordMessage(ctx, msg); err != nil {
		l.logger.Error("failed to record tool result message", "error", err)
	}
}

// nextSequenceID increments and returns the next sequence ID.
func (l *Loop) nextSequenceID() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sequenceID++
	return l.sequenceID
}

// recordErrorMessage records an error message to the conversation.
func (l *Loop) recordErrorMessage(ctx context.Context, errMsg string) {
	if l.recordMessage == nil {
		return
	}

	msg := Message{
		ConversationID: l.conversationID,
		Type:           MessageTypeError,
		SequenceID:     l.nextSequenceID(),
		Content:        errMsg,
		CreatedAt:      time.Now(),
	}
	if err := l.recordMessage(ctx, msg); err != nil {
		l.logger.Error("failed to record error message", "error", err)
	}
}

// buildMessages prepares the message list for an LLM request by optionally
// prepending the system prompt to the conversation history.
func (l *Loop) buildMessages(history []llm.Message) []llm.Message {
	if l.systemPrompt == "" {
		return history
	}

	messages := make([]llm.Message, 0, len(history)+1)
	messages = append(messages, llm.Message{
		Role:    llm.RoleSystem,
		Content: l.systemPrompt,
	})
	return append(messages, history...)
}

// buildToolDefinitions converts agent tools to LLM tool definitions.
func (l *Loop) buildToolDefinitions() []llm.Tool {
	llmTools := make([]llm.Tool, len(l.tools))
	for i, t := range l.tools {
		llmTools[i] = t.Tool
	}
	return llmTools
}

// accumulateUsage adds response usage statistics to the total.
func (l *Loop) accumulateUsage(usage llm.Usage) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.totalUsage.PromptTokens += usage.PromptTokens
	l.totalUsage.CompletionTokens += usage.CompletionTokens
	l.totalUsage.TotalTokens += usage.TotalTokens
}

// recordAssistantMessage adds the assistant response to history and records it.
func (l *Loop) recordAssistantMessage(ctx context.Context, resp *llm.ChatResponse) {
	assistantMessage := llm.Message{
		Role:      llm.RoleAssistant,
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	}
	seqID := l.appendToHistory(assistantMessage)

	if l.recordMessage == nil {
		return
	}

	msg := Message{
		ConversationID: l.conversationID,
		Type:           MessageTypeAssistant,
		SequenceID:     seqID,
		Content:        resp.Content,
		ToolCalls:      resp.ToolCalls,
		Usage:          &resp.Usage,
		CreatedAt:      time.Now(),
		LLMData:        &assistantMessage,
	}
	if err := l.recordMessage(ctx, msg); err != nil {
		l.logger.Error("failed to record assistant message", "error", err)
	}
}

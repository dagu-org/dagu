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
}

// Loop manages a conversation turn with an LLM including tool execution.
// Based on Shelley's loop/loop.go pattern.
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
	}
}

// QueueUserMessage adds a user message to the queue to be processed.
func (l *Loop) QueueUserMessage(message llm.Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messageQueue = append(l.messageQueue, message)
	l.logger.Debug("queued user message", "content_length", len(message.Content))
}

// GetUsage returns the total usage accumulated by this loop.
func (l *Loop) GetUsage() llm.Usage {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.totalUsage
}

// GetHistory returns a copy of the current conversation history.
func (l *Loop) GetHistory() []llm.Message {
	l.mu.Lock()
	defer l.mu.Unlock()
	historyCopy := make([]llm.Message, len(l.history))
	copy(historyCopy, l.history)
	return historyCopy
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
			for _, msg := range l.messageQueue {
				l.history = append(l.history, msg)
			}
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
	messages := append([]llm.Message(nil), l.history...)
	tools := l.tools
	systemPrompt := l.systemPrompt
	provider := l.provider
	model := l.model
	l.mu.Unlock()

	// Build messages with system prompt
	var llmMessages []llm.Message
	if systemPrompt != "" {
		llmMessages = append(llmMessages, llm.Message{
			Role:    llm.RoleSystem,
			Content: systemPrompt,
		})
	}
	llmMessages = append(llmMessages, messages...)

	// Build tool definitions for LLM
	llmTools := make([]llm.Tool, len(tools))
	for i, t := range tools {
		llmTools[i] = t.Tool
	}

	req := &llm.ChatRequest{
		Model:    model,
		Messages: llmMessages,
		Tools:    llmTools,
	}

	l.logger.Debug("sending LLM request",
		"message_count", len(llmMessages),
		"tool_count", len(llmTools),
		"model", model)

	// Set agent as working
	if l.onWorking != nil {
		l.onWorking(true)
	}

	// Add a timeout for the LLM request
	llmCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	resp, err := provider.Chat(llmCtx, req)
	if err != nil {
		// Record the error as a message
		l.recordErrorMessage(ctx, fmt.Sprintf("LLM request failed: %v", err))
		if l.onWorking != nil {
			l.onWorking(false)
		}
		return fmt.Errorf("LLM request failed: %w", err)
	}

	l.logger.Debug("received LLM response",
		"content_length", len(resp.Content),
		"finish_reason", resp.FinishReason,
		"tool_calls", len(resp.ToolCalls))

	// Update total usage
	l.mu.Lock()
	l.totalUsage.PromptTokens += resp.Usage.PromptTokens
	l.totalUsage.CompletionTokens += resp.Usage.CompletionTokens
	l.totalUsage.TotalTokens += resp.Usage.TotalTokens
	l.mu.Unlock()

	// Convert response to message and add to history
	assistantMessage := llm.Message{
		Role:      llm.RoleAssistant,
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	}

	l.mu.Lock()
	l.history = append(l.history, assistantMessage)
	l.sequenceID++
	seqID := l.sequenceID
	l.mu.Unlock()

	// Record assistant message
	if l.recordMessage != nil {
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

	// Handle tool calls if any
	// Note: Anthropic returns "tool_use" as stop_reason, OpenAI uses "tool_calls"
	if (resp.FinishReason == "tool_use" || resp.FinishReason == "tool_calls") && len(resp.ToolCalls) > 0 {
		l.logger.Debug("handling tool calls", "count", len(resp.ToolCalls))
		return l.handleToolCalls(ctx, resp.ToolCalls)
	}

	// End of turn - no more tool calls
	if l.onWorking != nil {
		l.onWorking(false)
	}

	return nil
}

// handleToolCalls processes tool calls from the LLM response.
func (l *Loop) handleToolCalls(ctx context.Context, toolCalls []llm.ToolCall) error {
	var toolResults []ToolResult

	for _, tc := range toolCalls {
		l.logger.Debug("executing tool", "name", tc.Function.Name, "id", tc.ID)

		// Find the tool
		var tool *AgentTool
		for _, t := range l.tools {
			if t.Function.Name == tc.Function.Name {
				tool = t
				break
			}
		}

		var result ToolOut
		if tool == nil {
			l.logger.Error("tool not found", "name", tc.Function.Name)
			result = ToolOut{
				Content: fmt.Sprintf("Tool '%s' not found", tc.Function.Name),
				IsError: true,
			}
		} else {
			// Parse arguments as JSON
			var input json.RawMessage
			if tc.Function.Arguments != "" {
				input = json.RawMessage(tc.Function.Arguments)
			} else {
				input = json.RawMessage("{}")
			}

			// Execute the tool
			toolCtx := ToolContext{WorkingDir: l.workingDir}
			result = tool.Run(toolCtx, input)
		}

		toolResults = append(toolResults, ToolResult{
			ToolCallID: tc.ID,
			Content:    result.Content,
			IsError:    result.IsError,
		})

		// Add tool result to history
		toolMessage := llm.Message{
			Role:       llm.RoleTool,
			Content:    result.Content,
			ToolCallID: tc.ID,
		}

		l.mu.Lock()
		l.history = append(l.history, toolMessage)
		l.sequenceID++
		seqID := l.sequenceID
		l.mu.Unlock()

		// Record tool result message
		if l.recordMessage != nil {
			msg := Message{
				ConversationID: l.conversationID,
				Type:           MessageTypeUser, // Tool results are from user perspective
				SequenceID:     seqID,
				ToolResults:    []ToolResult{{ToolCallID: tc.ID, Content: result.Content, IsError: result.IsError}},
				CreatedAt:      time.Now(),
				LLMData:        &toolMessage,
			}
			if err := l.recordMessage(ctx, msg); err != nil {
				l.logger.Error("failed to record tool result message", "error", err)
			}
		}
	}

	// Process another LLM request with the tool results
	return l.processLLMRequest(ctx)
}

// recordErrorMessage records an error message to the conversation.
func (l *Loop) recordErrorMessage(ctx context.Context, errMsg string) {
	if l.recordMessage == nil {
		return
	}

	l.mu.Lock()
	l.sequenceID++
	seqID := l.sequenceID
	l.mu.Unlock()

	msg := Message{
		ConversationID: l.conversationID,
		Type:           MessageTypeError,
		SequenceID:     seqID,
		Content:        errMsg,
		CreatedAt:      time.Now(),
	}
	if err := l.recordMessage(ctx, msg); err != nil {
		l.logger.Error("failed to record error message", "error", err)
	}
}

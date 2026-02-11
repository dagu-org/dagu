package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/backoff"
	"github.com/dagu-org/dagu/internal/llm"
)

const (
	// llmRetryInitialInterval is the initial backoff interval for LLM request retries.
	llmRetryInitialInterval = time.Second

	// idlePollingInterval is the interval for polling when no messages are queued.
	idlePollingInterval = 100 * time.Millisecond

	// llmRequestTimeout is the maximum time allowed for an LLM request.
	llmRequestTimeout = 5 * time.Minute

	// maxToolCallDepth limits nested tool call chains to prevent infinite recursion.
	// This can happen if an LLM continuously makes tool calls without producing a final response.
	maxToolCallDepth = 50
)

// MessageRecordFunc is called to record new messages to persistent storage.
type MessageRecordFunc func(ctx context.Context, message Message) error

// LoopConfig contains configuration for creating a Loop.
type LoopConfig struct {
	// Provider is the LLM provider for making requests.
	Provider llm.Provider
	// Model is the model ID to use for requests.
	Model string
	// History is the initial session history.
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
	// SessionID is the ID of the session.
	SessionID string
	// OnWorking is called when the working state changes.
	OnWorking func(working bool)
	// EmitUIAction is called when a tool wants to emit a UI action.
	EmitUIAction UIActionFunc
	// EmitUserPrompt is called when a tool wants to emit a user prompt.
	EmitUserPrompt EmitUserPromptFunc
	// WaitUserResponse blocks until user responds to a prompt.
	WaitUserResponse WaitUserResponseFunc
	// SafeMode enables approval prompts for dangerous commands when true.
	SafeMode bool
	// Hooks provides lifecycle callbacks for tool execution.
	Hooks *Hooks
	// UserID is the authenticated user's ID.
	UserID string
	// Username is the authenticated user's display name.
	Username string
	// IPAddress is the client's IP address.
	IPAddress string
}

// Loop manages a session turn with an LLM including tool execution.
type Loop struct {
	provider         llm.Provider
	model            string
	tools            []*AgentTool
	recordMessage    MessageRecordFunc
	history          []llm.Message
	messageQueue     []llm.Message
	totalUsage       llm.Usage
	mu               sync.Mutex
	logger           *slog.Logger
	systemPrompt     string
	workingDir       string
	sessionID        string
	onWorking        func(working bool)
	sequenceID       int64
	emitUIAction     UIActionFunc
	emitUserPrompt   EmitUserPromptFunc
	waitUserResponse WaitUserResponseFunc
	safeMode         bool
	hooks            *Hooks
	userID           string
	username         string
	ipAddress        string
}

// NewLoop creates a new Loop instance.
func NewLoop(config LoopConfig) *Loop {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Loop{
		provider:         config.Provider,
		model:            config.Model,
		history:          config.History,
		tools:            config.Tools,
		recordMessage:    config.RecordMessage,
		messageQueue:     make([]llm.Message, 0),
		logger:           logger,
		systemPrompt:     config.SystemPrompt,
		workingDir:       config.WorkingDir,
		sessionID:        config.SessionID,
		onWorking:        config.OnWorking,
		emitUIAction:     config.EmitUIAction,
		emitUserPrompt:   config.EmitUserPrompt,
		waitUserResponse: config.WaitUserResponse,
		safeMode:         config.SafeMode,
		hooks:            config.Hooks,
		userID:           config.UserID,
		username:         config.Username,
		ipAddress:        config.IPAddress,
	}
}

// QueueUserMessage adds a user message to the queue to be processed.
func (l *Loop) QueueUserMessage(message llm.Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messageQueue = append(l.messageQueue, message)
	l.logger.Info("queued user message", "queue_size", len(l.messageQueue))
}

// SetSafeMode updates the safe mode setting for this loop.
func (l *Loop) SetSafeMode(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.safeMode = enabled
}

// Go runs the session loop until the context is canceled.
func (l *Loop) Go(ctx context.Context) error {
	if l.provider == nil {
		return fmt.Errorf("no LLM provider configured")
	}

	l.logger.Info("starting session loop", "tools", len(l.tools))

	retrier := backoff.NewRetrier(backoff.NewExponentialBackoffPolicy(llmRetryInitialInterval))

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("session loop canceled")
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
			l.logger.Info("processing queued messages", "history_count", len(l.history))
			if err := l.processLLMRequest(ctx); err != nil {
				l.logger.Error("failed to process LLM request", "error", err)
				interval, _ := retrier.Next(err)
				l.sleepWithContext(ctx, interval)
				continue
			}
			retrier.Reset()
			l.logger.Info("finished processing queued messages")
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(idlePollingInterval):
			}
		}
	}
}

// sleepWithContext sleeps for the given duration or until the context is canceled.
func (l *Loop) sleepWithContext(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// processLLMRequest sends a request to the LLM and handles the response.
func (l *Loop) processLLMRequest(ctx context.Context) error {
	history := l.copyHistory()
	messages := l.buildMessages(history)
	tools := l.buildToolDefinitions()

	req := &llm.ChatRequest{
		Model:    l.model,
		Messages: messages,
		Tools:    tools,
	}

	l.logger.Debug("sending LLM request",
		"message_count", len(messages),
		"tool_count", len(tools),
		"model", l.model)

	l.setWorking(true)

	llmCtx, cancel := context.WithTimeout(ctx, llmRequestTimeout)
	defer cancel()

	resp, err := l.provider.Chat(llmCtx, req)
	if err != nil {
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
		l.logger.Info("handling tool calls", "count", len(resp.ToolCalls))
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

// copyHistory returns a thread-safe copy of the session history.
func (l *Loop) copyHistory() []llm.Message {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]llm.Message(nil), l.history...)
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
func (l *Loop) executeTool(ctx context.Context, tc llm.ToolCall) ToolOut {
	tool := GetToolByName(l.tools, tc.Function.Name)
	if tool == nil {
		l.logger.Error("tool not found", "name", tc.Function.Name)
		return toolError("Tool '%s' not found", tc.Function.Name)
	}

	input := json.RawMessage(tc.Function.Arguments)
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}

	info := ToolExecInfo{
		ToolName:       tc.Function.Name,
		Input:          input,
		SessionID: l.sessionID,
		UserID:         l.userID,
		Username:       l.username,
		IPAddress:      l.ipAddress,
		Audit:          tool.Audit,
	}

	if err := l.hooks.RunBeforeToolExec(ctx, info); err != nil {
		return toolError("Blocked by policy: %v", err)
	}

	l.mu.Lock()
	safeMode := l.safeMode
	l.mu.Unlock()

	result := tool.Run(ToolContext{
		Context:          ctx,
		WorkingDir:       l.workingDir,
		EmitUIAction:     l.emitUIAction,
		EmitUserPrompt:   l.emitUserPrompt,
		WaitUserResponse: l.waitUserResponse,
		SafeMode:         safeMode,
	}, input)

	l.hooks.RunAfterToolExec(ctx, info, result)

	return result
}

// handleToolCalls processes tool calls from the LLM response using iteration
// instead of recursion to prevent stack overflow with long tool call chains.
func (l *Loop) handleToolCalls(ctx context.Context, toolCalls []llm.ToolCall) error {
	for depth := range maxToolCallDepth {
		l.executeToolCalls(ctx, toolCalls, depth)

		resp, err := l.sendToolChainRequest(ctx, depth)
		if err != nil {
			return err
		}

		if len(resp.ToolCalls) == 0 {
			l.setWorking(false)
			return nil
		}

		l.logger.Info("continuing tool calls", "count", len(resp.ToolCalls), "depth", depth+1)
		toolCalls = resp.ToolCalls
	}

	l.setWorking(false)
	l.logger.Warn("max tool call depth reached", "depth", maxToolCallDepth)
	return fmt.Errorf("max tool call depth (%d) reached", maxToolCallDepth)
}

// executeToolCalls runs all tool calls at the current depth level.
func (l *Loop) executeToolCalls(ctx context.Context, toolCalls []llm.ToolCall, depth int) {
	for _, tc := range toolCalls {
		l.logger.Debug("executing tool", "name", tc.Function.Name, "id", tc.ID, "depth", depth)
		l.recordToolResult(ctx, tc, l.executeTool(ctx, tc))
	}
}

// sendToolChainRequest sends an LLM request after tool execution.
func (l *Loop) sendToolChainRequest(ctx context.Context, depth int) (*llm.ChatResponse, error) {
	history := l.copyHistory()
	messages := l.buildMessages(history)
	tools := l.buildToolDefinitions()

	req := &llm.ChatRequest{
		Model:    l.model,
		Messages: messages,
		Tools:    tools,
	}

	l.logger.Debug("sending LLM request (tool chain)",
		"message_count", len(messages),
		"tool_count", len(tools),
		"model", l.model,
		"depth", depth)

	l.setWorking(true)

	llmCtx, cancel := context.WithTimeout(ctx, llmRequestTimeout)
	defer cancel()

	resp, err := l.provider.Chat(llmCtx, req)
	if err != nil {
		l.recordErrorMessage(ctx, fmt.Sprintf("LLM request failed: %v", err))
		l.setWorking(false)
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	l.logger.Debug("received LLM response (tool chain)",
		"content_length", len(resp.Content),
		"finish_reason", resp.FinishReason,
		"tool_calls", len(resp.ToolCalls),
		"depth", depth)

	l.accumulateUsage(resp.Usage)
	l.recordAssistantMessage(ctx, resp)
	return resp, nil
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
		SessionID: l.sessionID,
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

// recordErrorMessage records an error message to the session.
func (l *Loop) recordErrorMessage(ctx context.Context, errMsg string) {
	if l.recordMessage == nil {
		return
	}

	msg := Message{
		SessionID: l.sessionID,
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
// prepending the system prompt to the session history.
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
		SessionID: l.sessionID,
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

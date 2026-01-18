// Package chat provides an executor for chat (LLM-based conversation) steps.
package chat

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/masking"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	llmpkg "github.com/dagu-org/dagu/internal/llm"

	// Import all providers to register them
	_ "github.com/dagu-org/dagu/internal/llm/allproviders"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var _ executor.Executor = (*Executor)(nil)
var _ executor.ChatMessageHandler = (*Executor)(nil)
var _ executor.SubRunProvider = (*Executor)(nil)

// Executor implements the executor.Executor interface for chat steps.
type Executor struct {
	stdout          io.Writer
	stderr          io.Writer
	step            core.Step
	providerType    llmpkg.ProviderType
	apiKeyEnvVar    string
	messages        []exec.LLMMessage
	contextMessages []exec.LLMMessage
	savedMessages   []exec.LLMMessage

	// Tool calling support
	toolRegistry *ToolRegistry
	toolExecutor *ToolExecutor

	// Collected sub-runs from tool executions for UI drill-down
	collectedSubRuns []exec.SubDAGRun
}

// newChatExecutor creates a new chat executor from a step configuration.
func newChatExecutor(ctx context.Context, step core.Step) (executor.Executor, error) {
	if step.LLM == nil {
		return nil, fmt.Errorf("llm configuration is required for chat executor")
	}

	cfg := step.LLM

	// Parse provider type (required field, validated in spec)
	providerType, err := llmpkg.ParseProviderType(cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("invalid provider: %w", err)
	}

	// Determine which environment variable to use for API key
	apiKeyEnvVar := cfg.APIKeyName
	if apiKeyEnvVar == "" {
		apiKeyEnvVar = llmpkg.DefaultAPIKeyEnvVar(providerType)
	}

	// Convert messages from core.LLMMessage to execution.LLMMessage
	// Messages are now at step level, not inside LLM config
	messages := make([]exec.LLMMessage, 0, len(step.Messages)+1)

	// Add system message from config if specified
	if cfg.System != "" {
		messages = append(messages, exec.LLMMessage{
			Role:    core.LLMRoleSystem,
			Content: cfg.System,
		})
	}

	// Add step-level messages
	for _, msg := range step.Messages {
		messages = append(messages, exec.LLMMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	e := &Executor{
		stdout:       os.Stdout,
		stderr:       os.Stderr,
		step:         step,
		providerType: providerType,
		apiKeyEnvVar: apiKeyEnvVar,
		messages:     messages,
	}

	// Initialize tool registry if tools are configured
	if cfg.HasTools() {
		registry, err := NewToolRegistry(ctx, cfg.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tool registry: %w", err)
		}
		e.toolRegistry = registry
	}

	return e, nil
}

// SetStdout sets the stdout writer.
func (e *Executor) SetStdout(out io.Writer) {
	e.stdout = out
}

// SetStderr sets the stderr writer.
func (e *Executor) SetStderr(out io.Writer) {
	e.stderr = out
}

// Kill terminates any running tool DAG executions.
func (e *Executor) Kill(sig os.Signal) error {
	if e.toolExecutor != nil {
		return e.toolExecutor.Kill(sig)
	}
	return nil
}

// SetContext sets the conversation context from prior steps.
func (e *Executor) SetContext(messages []exec.LLMMessage) {
	e.contextMessages = messages
}

// GetMessages returns the complete conversation messages after execution.
// This includes inherited messages, step messages, and the assistant response.
func (e *Executor) GetMessages() []exec.LLMMessage {
	return e.savedMessages
}

// GetSubRuns returns the collected sub-DAG runs from tool executions.
// This implements the SubRunProvider interface for UI drill-down functionality.
func (e *Executor) GetSubRuns() []exec.SubDAGRun {
	return e.collectedSubRuns
}

// buildMessageList orders messages so step's system message takes precedence over context.
func buildMessageList(stepMsgs, contextMsgs []exec.LLMMessage) []exec.LLMMessage {
	var result []exec.LLMMessage
	var stepSystemMsg *exec.LLMMessage
	var stepOtherMsgs []exec.LLMMessage

	for i := range stepMsgs {
		if stepMsgs[i].Role == exec.RoleSystem {
			stepSystemMsg = &stepMsgs[i]
		} else {
			stepOtherMsgs = append(stepOtherMsgs, stepMsgs[i])
		}
	}

	if stepSystemMsg != nil {
		result = append(result, *stepSystemMsg)
	}
	result = append(result, contextMsgs...)
	result = append(result, stepOtherMsgs...)

	return exec.DeduplicateSystemMessages(result)
}

// toLLMMessages converts execution.LLMMessage to llmpkg.Message for provider calls.
func toLLMMessages(msgs []exec.LLMMessage) []llmpkg.Message {
	result := make([]llmpkg.Message, len(msgs))
	for i, msg := range msgs {
		result[i] = llmpkg.Message{
			Role:       llmpkg.Role(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
	}
	return result
}

// toThinkingRequest converts core.ThinkingConfig to llmpkg.ThinkingRequest.
func toThinkingRequest(cfg *core.ThinkingConfig) *llmpkg.ThinkingRequest {
	if cfg == nil {
		return nil
	}
	return &llmpkg.ThinkingRequest{
		Enabled:         cfg.Enabled,
		Effort:          llmpkg.ThinkingEffort(cfg.Effort),
		BudgetTokens:    cfg.BudgetTokens,
		IncludeInOutput: cfg.IncludeInOutput,
	}
}

// normalizeEnvVarExpr converts an environment variable reference to ${VAR} format.
// Handles: VAR → ${VAR}, $VAR → ${VAR}, ${VAR} → ${VAR}, "" → ""
func normalizeEnvVarExpr(expr string) string {
	if expr == "" {
		return ""
	}
	if strings.HasPrefix(expr, "${") {
		// Already in ${VAR} format, use as-is
		return expr
	}
	if strings.HasPrefix(expr, "$") {
		// Convert $VAR to ${VAR}
		return "${" + strings.TrimPrefix(expr, "$") + "}"
	}
	// Plain variable name, wrap in ${...}
	return "${" + expr + "}"
}

// createProvider creates the LLM provider with evaluated config values.
// This is called at runtime to support variable substitution in config fields.
func (e *Executor) createProvider(ctx context.Context) (llmpkg.Provider, error) {
	cfg := e.step.LLM

	// Evaluate API key from environment variable (supports ${VAR} substitution)
	var apiKey string
	if e.apiKeyEnvVar != "" {
		apiKeyExpr := normalizeEnvVarExpr(e.apiKeyEnvVar)
		var err error
		apiKey, err = runtime.EvalString(ctx, apiKeyExpr)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate API key: %w", err)
		}
	}

	// Evaluate base URL if specified
	baseURL := cfg.BaseURL
	if baseURL != "" {
		var err error
		baseURL, err = runtime.EvalString(ctx, baseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate baseURL: %w", err)
		}
	}

	// Use default base URL if not specified
	if baseURL == "" {
		baseURL = llmpkg.DefaultBaseURL(e.providerType)
	}

	// Build provider config
	providerCfg := llmpkg.Config{
		APIKey:          apiKey,
		BaseURL:         baseURL,
		Timeout:         5 * time.Minute,
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
	}

	// Create provider
	provider, err := llmpkg.NewProvider(e.providerType, providerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	return provider, nil
}

// evalMessages evaluates variable substitution in message content.
func evalMessages(ctx context.Context, msgs []exec.LLMMessage) ([]exec.LLMMessage, error) {
	result := make([]exec.LLMMessage, len(msgs))
	for i, msg := range msgs {
		content, err := runtime.EvalString(ctx, msg.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate message content: %w", err)
		}
		result[i] = exec.LLMMessage{
			Role:    msg.Role,
			Content: content,
		}
	}
	return result, nil
}

// maskSecretsForProvider masks secret values in messages before sending to LLM provider.
// This prevents secrets from being leaked to external LLM APIs.
func maskSecretsForProvider(ctx context.Context, msgs []exec.LLMMessage) []exec.LLMMessage {
	// Use EnvScope.AllSecrets() for unified source tracking
	rCtx := runtime.GetDAGContext(ctx)
	if rCtx.EnvScope == nil {
		return msgs
	}
	secrets := rCtx.EnvScope.AllSecrets()
	if len(secrets) == 0 {
		return msgs
	}

	envPairs := make([]string, 0, len(secrets))
	for k, v := range secrets {
		envPairs = append(envPairs, k+"="+v)
	}

	masker := masking.NewMasker(masking.SourcedEnvVars{Secrets: envPairs})

	result := make([]exec.LLMMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = exec.LLMMessage{
			Role:     msg.Role,
			Content:  masker.MaskString(msg.Content),
			Metadata: msg.Metadata,
		}
	}
	return result
}

// Run executes the chat request.
func (e *Executor) Run(ctx context.Context) error {
	// Create provider with evaluated config (supports variable substitution)
	provider, err := e.createProvider(ctx)
	if err != nil {
		return err
	}

	evaluatedMessages, err := evalMessages(ctx, e.messages)
	if err != nil {
		return err
	}

	allMessages := buildMessageList(evaluatedMessages, e.contextMessages)

	// Dispatch to tool-enabled execution if tools are configured
	if e.toolRegistry != nil && e.toolRegistry.HasTools() {
		return e.runWithTools(ctx, provider, allMessages)
	}

	// Standard execution without tools
	return e.runSimple(ctx, provider, allMessages)
}

// runSimple executes a chat request without tool calling.
func (e *Executor) runSimple(ctx context.Context, provider llmpkg.Provider, allMessages []exec.LLMMessage) error {
	cfg := e.step.LLM
	maskedForProvider := maskSecretsForProvider(ctx, allMessages)

	req := &llmpkg.ChatRequest{
		Model:       cfg.Model,
		Messages:    toLLMMessages(maskedForProvider),
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		TopP:        cfg.TopP,
		Thinking:    toThinkingRequest(cfg.Thinking),
	}

	var responseContent string
	var usage *llmpkg.Usage

	// Execute request (streaming or non-streaming)
	if cfg.StreamEnabled() {
		events, err := provider.ChatStream(ctx, req)
		if err != nil {
			return fmt.Errorf("chat stream request failed: %w", err)
		}

		// Collect response content while streaming to stdout
		for event := range events {
			if event.Error != nil {
				return fmt.Errorf("chat stream error: %w", event.Error)
			}
			if event.Delta != "" {
				responseContent += event.Delta
				if _, err := e.stdout.Write([]byte(event.Delta)); err != nil {
					logger.Error(ctx, "failed to write streaming response", tag.Error(err))
				}
			}
			// Capture usage from final event
			if event.Usage != nil {
				usage = event.Usage
			}
		}
		// Add newline after streaming response
		if _, err := e.stdout.Write([]byte("\n")); err != nil {
			logger.Error(ctx, "failed to write newline", tag.Error(err))
		}
	} else {
		resp, err := provider.Chat(ctx, req)
		if err != nil {
			return fmt.Errorf("chat request failed: %w", err)
		}
		responseContent = resp.Content
		usage = &resp.Usage
		if _, err := fmt.Fprintln(e.stdout, responseContent); err != nil {
			logger.Error(ctx, "failed to write response", tag.Error(err))
		}
	}

	// Build metadata for the assistant response
	metadata := &exec.LLMMessageMetadata{
		Provider: cfg.Provider,
		Model:    cfg.Model,
	}
	if usage != nil {
		metadata.PromptTokens = usage.PromptTokens
		metadata.CompletionTokens = usage.CompletionTokens
		metadata.TotalTokens = usage.TotalTokens
	}

	// Save full conversation (inherited + step messages + response)
	e.savedMessages = append(allMessages, exec.LLMMessage{
		Role:     exec.RoleAssistant,
		Content:  responseContent,
		Metadata: metadata,
	})

	return nil
}

// runWithTools executes a chat request with tool calling support.
// It implements a multi-turn loop where:
// 1. Send messages + tools to LLM
// 2. If LLM requests tool calls, execute them
// 3. Add tool results to conversation
// 4. Repeat until LLM provides final response (no more tool calls) or max iterations
func (e *Executor) runWithTools(ctx context.Context, provider llmpkg.Provider, allMessages []exec.LLMMessage) error {
	cfg := e.step.LLM
	maxIterations := cfg.GetMaxToolIterations()

	// Initialize tool executor for running DAGs
	rCtx := runtime.GetDAGContext(ctx)
	workDir := ""
	if rCtx.DAG != nil {
		workDir = rCtx.DAG.Location
	}
	e.toolExecutor = NewToolExecutor(e.toolRegistry, workDir)

	// Get tools in LLM format
	tools := e.toolRegistry.ToLLMTools()

	logger.Info(ctx, "Starting tool-enabled chat execution",
		slog.Int("tool_count", len(tools)),
		slog.Int("max_iterations", maxIterations),
	)

	// Working copy of messages for the tool loop
	conversationMessages := make([]exec.LLMMessage, len(allMessages))
	copy(conversationMessages, allMessages)

	for iteration := 0; iteration < maxIterations; iteration++ {
		logger.Debug(ctx, "Tool loop iteration",
			slog.Int("iteration", iteration+1),
			slog.Int("message_count", len(conversationMessages)),
		)

		// Mask secrets before sending to provider
		maskedForProvider := maskSecretsForProvider(ctx, conversationMessages)

		// Build request with tools
		req := &llmpkg.ChatRequest{
			Model:       cfg.Model,
			Messages:    toLLMMessages(maskedForProvider),
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxTokens,
			TopP:        cfg.TopP,
			Thinking:    toThinkingRequest(cfg.Thinking),
			Tools:       tools,
			ToolChoice:  "auto",
		}

		// Execute request (non-streaming for tool calling - streaming complicates tool call detection)
		resp, err := provider.Chat(ctx, req)
		if err != nil {
			return fmt.Errorf("chat request failed: %w", err)
		}

		// Check if the response contains tool calls
		if len(resp.ToolCalls) == 0 {
			// No tool calls - this is the final response
			logger.Info(ctx, "LLM provided final response (no tool calls)",
				slog.Int("iterations_used", iteration+1),
			)

			// Write the final response to stdout
			if resp.Content != "" {
				if _, err := fmt.Fprintln(e.stdout, resp.Content); err != nil {
					logger.Error(ctx, "failed to write response", tag.Error(err))
				}
			}

			// Build metadata for the assistant response
			metadata := &exec.LLMMessageMetadata{
				Provider:         cfg.Provider,
				Model:            cfg.Model,
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			}

			// Save full conversation including tool interactions
			e.savedMessages = append(conversationMessages, exec.LLMMessage{
				Role:     exec.RoleAssistant,
				Content:  resp.Content,
				Metadata: metadata,
			})

			return nil
		}

		// LLM requested tool calls - execute them
		logger.Info(ctx, "LLM requested tool calls",
			slog.Int("tool_call_count", len(resp.ToolCalls)),
		)

		// Add assistant message with tool calls to conversation
		assistantMsg := exec.LLMMessage{
			Role:    exec.RoleAssistant,
			Content: resp.Content,
		}
		conversationMessages = append(conversationMessages, assistantMsg)

		// Execute each tool call and collect results
		toolCallResults := e.toolExecutor.ExecuteToolCalls(ctx, resp.ToolCalls)

		// Add tool results to conversation and collect sub-runs for UI
		for _, tcr := range toolCallResults {
			result := tcr.Result
			toolMsg := exec.LLMMessage{
				Role:       exec.RoleTool,
				Content:    result.Content,
				ToolCallID: result.ToolCallID,
			}
			if result.Error != "" {
				toolMsg.Content = fmt.Sprintf("Error: %s", result.Error)
			}
			conversationMessages = append(conversationMessages, toolMsg)

			// Collect sub-run info for UI drill-down (even for failed executions)
			if tcr.SubRun.DAGRunID != "" {
				e.collectedSubRuns = append(e.collectedSubRuns, tcr.SubRun)
			}

			// Log tool result (truncated for readability)
			content := result.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			logger.Info(ctx, "Tool execution result",
				tag.Tool(result.Name),
				tag.ToolCallID(result.ToolCallID),
				slog.String("content_preview", content),
			)
		}
	}

	// Max iterations reached - return with warning
	logger.Warn(ctx, "Max tool iterations reached",
		slog.Int("max_iterations", maxIterations),
	)

	// Get the last assistant response if any
	var lastContent string
	for i := len(conversationMessages) - 1; i >= 0; i-- {
		if conversationMessages[i].Role == exec.RoleAssistant {
			lastContent = conversationMessages[i].Content
			break
		}
	}

	if lastContent == "" {
		lastContent = fmt.Sprintf("[Max tool iterations (%d) reached. The LLM may not have provided a complete response.]", maxIterations)
	}

	if _, err := fmt.Fprintln(e.stdout, lastContent); err != nil {
		logger.Error(ctx, "failed to write response", tag.Error(err))
	}

	// Save conversation state
	e.savedMessages = conversationMessages

	return nil
}


func init() {
	executor.RegisterExecutor(core.ExecutorTypeChat, newChatExecutor, nil, core.ExecutorCapabilities{
		LLM: true,
		// All others false - chat doesn't support command, script, shell, container, subdag
	})
}

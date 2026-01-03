// Package chat provides an executor for chat (LLM-based conversation) steps.
package chat

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	llmpkg "github.com/dagu-org/dagu/internal/llm"
	// Import all providers to register them
	_ "github.com/dagu-org/dagu/internal/llm/allproviders"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var _ executor.Executor = (*Executor)(nil)
var _ executor.ChatMessageHandler = (*Executor)(nil)

// Executor implements the executor.Executor interface for chat steps.
type Executor struct {
	stdout          io.Writer
	stderr          io.Writer
	step            core.Step
	provider        llmpkg.Provider
	messages        []execution.LLMMessage
	contextMessages []execution.LLMMessage
	savedMessages   []execution.LLMMessage
}

// newChatExecutor creates a new chat executor from a step configuration.
func newChatExecutor(_ context.Context, step core.Step) (executor.Executor, error) {
	if step.LLM == nil {
		return nil, fmt.Errorf("llm configuration is required for chat executor")
	}

	cfg := step.LLM

	// Parse provider type (required field, validated in spec)
	providerType, err := llmpkg.ParseProviderType(cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("invalid provider: %w", err)
	}

	// Build provider config
	providerCfg := llmpkg.Config{
		APIKey:          cfg.APIKey,
		BaseURL:         cfg.BaseURL,
		Timeout:         5 * time.Minute,
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
	}

	// Use default API key from environment if not specified
	if providerCfg.APIKey == "" {
		envVar := llmpkg.DefaultAPIKeyEnvVar(providerType)
		if envVar != "" {
			providerCfg.APIKey = os.Getenv(envVar)
		}
	}

	// Use default base URL if not specified
	if providerCfg.BaseURL == "" {
		providerCfg.BaseURL = llmpkg.DefaultBaseURL(providerType)
	}

	// Create provider
	provider, err := llmpkg.NewProvider(providerType, providerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Convert messages from core.LLMMessage to execution.LLMMessage
	// Messages are now at step level, not inside LLM config
	messages := make([]execution.LLMMessage, 0, len(step.Messages)+1)

	// Add system message from config if specified
	if cfg.System != "" {
		messages = append(messages, execution.LLMMessage{
			Role:    core.LLMRoleSystem,
			Content: cfg.System,
		})
	}

	// Add step-level messages
	for _, msg := range step.Messages {
		messages = append(messages, execution.LLMMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return &Executor{
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		step:     step,
		provider: provider,
		messages: messages,
	}, nil
}

// SetStdout sets the stdout writer.
func (e *Executor) SetStdout(out io.Writer) {
	e.stdout = out
}

// SetStderr sets the stderr writer.
func (e *Executor) SetStderr(out io.Writer) {
	e.stderr = out
}

// Kill is a no-op for chat executor (requests are context-cancelled).
func (e *Executor) Kill(_ os.Signal) error {
	return nil
}

// SetContext sets the conversation context from prior steps.
func (e *Executor) SetContext(messages []execution.LLMMessage) {
	e.contextMessages = messages
}

// GetMessages returns the complete conversation messages after execution.
// This includes inherited messages, step messages, and the assistant response.
func (e *Executor) GetMessages() []execution.LLMMessage {
	return e.savedMessages
}

// toLLMMessages converts execution.LLMMessage to llmpkg.Message for provider calls.
func toLLMMessages(msgs []execution.LLMMessage) []llmpkg.Message {
	result := make([]llmpkg.Message, len(msgs))
	for i, msg := range msgs {
		result[i] = llmpkg.Message{
			Role:    llmpkg.Role(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

// evalMessages evaluates variable substitution in message content.
func evalMessages(ctx context.Context, msgs []execution.LLMMessage) ([]execution.LLMMessage, error) {
	result := make([]execution.LLMMessage, len(msgs))
	for i, msg := range msgs {
		content, err := runtime.EvalString(ctx, msg.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate message content: %w", err)
		}
		result[i] = execution.LLMMessage{
			Role:    msg.Role,
			Content: content,
		}
	}
	return result, nil
}

// Run executes the chat request.
func (e *Executor) Run(ctx context.Context) error {
	cfg := e.step.LLM

	// Evaluate variable substitution in this step's messages
	evaluatedMessages, err := evalMessages(ctx, e.messages)
	if err != nil {
		return err
	}

	// Build complete message list: context + this step's messages
	var allMessages []execution.LLMMessage
	allMessages = append(allMessages, e.contextMessages...)
	allMessages = append(allMessages, evaluatedMessages...)

	// Deduplicate system messages (keep only the first one)
	allMessages = execution.DeduplicateSystemMessages(allMessages)

	// Build chat request - convert to provider format only at boundary
	req := &llmpkg.ChatRequest{
		Model:       cfg.Model,
		Messages:    toLLMMessages(allMessages),
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		TopP:        cfg.TopP,
	}

	var responseContent string
	var usage *llmpkg.Usage

	// Execute request (streaming or non-streaming)
	if cfg.StreamEnabled() {
		events, err := e.provider.ChatStream(ctx, req)
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
				_, _ = e.stdout.Write([]byte(event.Delta))
			}
			// Capture usage from final event
			if event.Usage != nil {
				usage = event.Usage
			}
		}
		// Add newline after streaming response
		_, _ = e.stdout.Write([]byte("\n"))
	} else {
		resp, err := e.provider.Chat(ctx, req)
		if err != nil {
			return fmt.Errorf("chat request failed: %w", err)
		}
		responseContent = resp.Content
		usage = &resp.Usage
		_, _ = fmt.Fprintln(e.stdout, responseContent)
	}

	// Build metadata for the assistant response
	metadata := &execution.LLMMessageMetadata{
		Provider: cfg.Provider,
		Model:    cfg.Model,
	}
	if usage != nil {
		metadata.PromptTokens = usage.PromptTokens
		metadata.CompletionTokens = usage.CompletionTokens
		metadata.TotalTokens = usage.TotalTokens
	}

	// Save full conversation (inherited + step messages + response)
	e.savedMessages = append(allMessages, execution.LLMMessage{
		Role:     execution.RoleAssistant,
		Content:  responseContent,
		Metadata: metadata,
	})

	return nil
}

func init() {
	executor.RegisterExecutor(core.ExecutorTypeChat, newChatExecutor, nil, core.ExecutorCapabilities{
		LLM: true,
		// All others false - chat doesn't support command, script, shell, container, subdag
	})
}

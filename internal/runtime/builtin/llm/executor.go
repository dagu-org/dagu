// Package llm provides an executor for LLM (Large Language Model) steps.
package llm

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
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var _ executor.Executor = (*Executor)(nil)

// Executor implements the executor.Executor interface for LLM steps.
type Executor struct {
	stdout            io.Writer
	stderr            io.Writer
	step              core.Step
	provider          llmpkg.Provider
	messages          []llmpkg.Message
	inheritedMessages []llmpkg.Message
	savedMessages     []llmpkg.Message
}

// newLLMExecutor creates a new LLM executor from a step configuration.
func newLLMExecutor(_ context.Context, step core.Step) (executor.Executor, error) {
	if step.LLM == nil {
		return nil, fmt.Errorf("LLM configuration is required")
	}

	cfg := step.LLM

	// Determine provider type
	providerType := llmpkg.ProviderOpenAI // Default
	if cfg.Provider != "" {
		var err error
		providerType, err = llmpkg.ParseProviderType(cfg.Provider)
		if err != nil {
			return nil, fmt.Errorf("invalid provider: %w", err)
		}
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

	// Convert messages from core.LLMMessage to llmpkg.Message
	messages := make([]llmpkg.Message, len(cfg.Messages))
	for i, msg := range cfg.Messages {
		messages[i] = llmpkg.Message{
			Role:    llmpkg.Role(msg.Role),
			Content: msg.Content,
		}
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

// Kill is a no-op for LLM executor (requests are context-cancelled).
func (e *Executor) Kill(_ os.Signal) error {
	return nil
}

// SetInheritedMessages sets the messages inherited from dependent steps.
func (e *Executor) SetInheritedMessages(messages []execution.LLMMessage) {
	e.inheritedMessages = toLLMMessages(messages)
}

// GetMessages returns the complete conversation messages after execution.
// This includes inherited messages, step messages, and the assistant response.
func (e *Executor) GetMessages() []execution.LLMMessage {
	return toExecutionMessages(e.savedMessages)
}

// toLLMMessages converts execution.LLMMessage to llmpkg.Message.
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

// toExecutionMessages converts llmpkg.Message to execution.LLMMessage.
func toExecutionMessages(msgs []llmpkg.Message) []execution.LLMMessage {
	result := make([]execution.LLMMessage, len(msgs))
	for i, msg := range msgs {
		result[i] = execution.LLMMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

// Run executes the LLM request.
func (e *Executor) Run(ctx context.Context) error {
	cfg := e.step.LLM

	// Build complete message list
	var allMessages []llmpkg.Message

	// Add inherited messages if history is enabled
	if cfg.HistoryEnabled() && len(e.inheritedMessages) > 0 {
		allMessages = append(allMessages, e.inheritedMessages...)
	}

	// Append this step's messages
	allMessages = append(allMessages, e.messages...)

	// Deduplicate system messages (keep only the first one)
	allMessages = toLLMMessages(execution.DeduplicateSystemMessages(toExecutionMessages(allMessages)))

	// Build chat request
	req := &llmpkg.ChatRequest{
		Model:       cfg.Model,
		Messages:    allMessages,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
		TopP:        cfg.TopP,
	}

	var responseContent string

	// Execute request (streaming or non-streaming)
	if cfg.StreamEnabled() {
		events, err := e.provider.ChatStream(ctx, req)
		if err != nil {
			return fmt.Errorf("LLM stream request failed: %w", err)
		}

		// Collect response content while streaming to stdout
		for event := range events {
			if event.Error != nil {
				return fmt.Errorf("LLM stream error: %w", event.Error)
			}
			if event.Delta != "" {
				responseContent += event.Delta
				_, _ = e.stdout.Write([]byte(event.Delta))
			}
		}
		// Add newline after streaming response
		_, _ = e.stdout.Write([]byte("\n"))
	} else {
		resp, err := e.provider.Chat(ctx, req)
		if err != nil {
			return fmt.Errorf("LLM request failed: %w", err)
		}
		responseContent = resp.Content
		_, _ = fmt.Fprintln(e.stdout, responseContent)
	}

	// Save messages (including assistant response) for persistence
	if cfg.HistoryEnabled() {
		e.savedMessages = append(allMessages, llmpkg.Message{
			Role:    llmpkg.RoleAssistant,
			Content: responseContent,
		})
	}

	return nil
}

func init() {
	executor.RegisterExecutor(core.ExecutorTypeLLM, newLLMExecutor, nil, core.ExecutorCapabilities{
		LLM: true,
		// All others false - LLM doesn't support command, script, shell, container, subdag
	})
}

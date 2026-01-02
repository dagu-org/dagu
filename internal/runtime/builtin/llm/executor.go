// Package llm provides an executor for LLM (Large Language Model) steps.
package llm

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	llmpkg "github.com/dagu-org/dagu/internal/llm"
	// Import all providers to register them
	_ "github.com/dagu-org/dagu/internal/llm/allproviders"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var _ executor.Executor = (*Executor)(nil)

// Executor implements the executor.Executor interface for LLM steps.
type Executor struct {
	stdout     io.Writer
	stderr     io.Writer
	step       core.Step
	provider   llmpkg.Provider
	messages   []llmpkg.Message
	historyDir string
	depends    []string
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
		depends:  step.Depends,
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

// SetHistoryDir sets the directory for history files.
func (e *Executor) SetHistoryDir(dir string) {
	e.historyDir = dir
}

// Run executes the LLM request.
func (e *Executor) Run(ctx context.Context) error {
	cfg := e.step.LLM

	// Start with messages from the step
	allMessages := make([]llmpkg.Message, 0, len(e.messages))

	// Load history from dependent steps if enabled
	if cfg.HistoryEnabled() && e.historyDir != "" && len(e.depends) > 0 {
		// Load history from the first dependency that has history
		// (dependencies are ordered, so we use the last one in the chain)
		for i := len(e.depends) - 1; i >= 0; i-- {
			depHistory, err := ReadDependentHistory(e.historyDir, e.depends[i])
			if err != nil {
				_, _ = fmt.Fprintf(e.stderr, "Warning: failed to read history from %s: %v\n", e.depends[i], err)
				continue
			}
			if len(depHistory) > 0 {
				allMessages = append(allMessages, depHistory...)
				break
			}
		}
	}

	// Append this step's messages
	allMessages = append(allMessages, e.messages...)

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

	// Save history if enabled
	if cfg.HistoryEnabled() && e.historyDir != "" {
		// Append assistant response to messages
		allMessages = append(allMessages, llmpkg.Message{
			Role:    llmpkg.RoleAssistant,
			Content: responseContent,
		})

		hf := NewHistoryFile(e.historyDir, e.step.Name)
		if err := hf.Write(allMessages); err != nil {
			_, _ = fmt.Fprintf(e.stderr, "Warning: failed to save history: %v\n", err)
		}
	}

	return nil
}

func init() {
	executor.RegisterExecutor(core.ExecutorTypeLLM, newLLMExecutor, nil, core.ExecutorCapabilities{
		LLM: true,
		// All others false - LLM doesn't support command, script, shell, container, subdag
	})
}

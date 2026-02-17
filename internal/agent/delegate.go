package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/google/uuid"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           "delegate",
		Label:          "Delegate",
		Description:    "Spawn sub-agents for parallel tasks",
		DefaultEnabled: true,
		Factory:        func(_ ToolConfig) *AgentTool { return NewDelegateTool() },
	})
}

const (
	// delegateToolName is the name of the delegate tool.
	delegateToolName = "delegate"

	// maxConcurrentDelegates is the maximum number of sub-agents that can run in parallel.
	maxConcurrentDelegates = 8

	// defaultDelegateMaxIterations is the default max iterations for a sub-agent.
	defaultDelegateMaxIterations = 20
)

// delegateInput is the parsed input for the delegate tool.
type delegateInput struct {
	Task          string `json:"task"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

// NewDelegateTool creates a delegate tool for spawning sub-agent loops.
func NewDelegateTool() *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        delegateToolName,
				Description: fmt.Sprintf("Spawn a sub-agent for a focused sub-task. The sub-agent works independently with the same tools and returns a summary. Use when a task benefits from dedicated, parallel execution. You can delegate up to %d tasks simultaneously.", maxConcurrentDelegates),
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"task": map[string]any{
							"type":        "string",
							"description": "Description of the sub-task for the sub-agent to complete",
						},
						"max_iterations": map[string]any{
							"type":        "integer",
							"description": "Maximum number of tool-call rounds (default: 20)",
						},
					},
					"required": []string{"task"},
				},
			},
		},
		Run: delegateRun,
	}
}

// delegateRun executes the delegate tool by spawning a sub-agent loop.
func delegateRun(ctx ToolContext, input json.RawMessage) ToolOut {
	if ctx.Delegate == nil {
		return toolError("Delegate capability is not available in this context")
	}

	var args delegateInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Invalid input: %v", err)
	}

	if args.Task == "" {
		return toolError("Task description is required")
	}

	maxIterations := defaultDelegateMaxIterations
	if args.MaxIterations > 0 {
		maxIterations = args.MaxIterations
	}

	delegateID := uuid.New().String()
	dc := ctx.Delegate

	logger := dc.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("delegate_id", delegateID, "task", truncate(args.Task, 100))

	// Persist sub-session in the store.
	if dc.SessionStore != nil {
		now := time.Now()
		subSession := &Session{
			ID:              delegateID,
			UserID:          dc.UserID,
			ParentSessionID: dc.ParentID,
			DelegateTask:    args.Task,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := dc.SessionStore.CreateSession(ctx.Context, subSession); err != nil {
			return toolError("Failed to create sub-session: %v", err)
		}
	}

	// Build onMessage callback for persistence.
	var onMessage func(msgCtx context.Context, msg Message) error
	if dc.SessionStore != nil {
		onMessage = func(msgCtx context.Context, msg Message) error {
			return dc.SessionStore.AddMessage(msgCtx, delegateID, &msg)
		}
	}

	// Create a sub-SessionManager for SSE streaming of the delegate's messages.
	subMgr := NewSessionManager(SessionManagerConfig{
		ID:              delegateID,
		UserID:          dc.UserID,
		Logger:          logger,
		WorkingDir:      ctx.WorkingDir,
		OnMessage:       onMessage,
		ParentSessionID: dc.ParentID,
		DelegateTask:    args.Task,
	})

	// Register sub-session for SSE streaming.
	if dc.RegisterSubSession != nil {
		dc.RegisterSubSession(delegateID, subMgr)
	}

	// Notify parent that delegate started.
	if dc.NotifyParent != nil {
		dc.NotifyParent(StreamResponse{
			DelegateEvent: &DelegateEvent{
				Type:       "started",
				DelegateID: delegateID,
				Task:       args.Task,
			},
		})
	}

	// Filter out the delegate tool from child tools to prevent recursion.
	childTools := filterOutTool(dc.Tools, delegateToolName)

	// Create a cancellable context for the child loop.
	childCtx, cancelChild := context.WithCancel(ctx.Context)
	defer cancelChild()

	// Track last assistant content and errors for result extraction.
	var lastAssistantContent string
	var lastError string
	iteration := 0

	// Build RecordMessage that publishes to sub-SessionManager's SubPub and captures results.
	captureResult := func(msg Message) {
		if msg.Type == MessageTypeAssistant && msg.Content != "" {
			lastAssistantContent = msg.Content
		}
		if msg.Type == MessageTypeError {
			lastError = msg.Content
		}
	}

	recordMessage := func(msgCtx context.Context, msg Message) error {
		captureResult(msg)
		return subMgr.RecordExternalMessage(msgCtx, msg)
	}

	loop := NewLoop(LoopConfig{
		Provider:      dc.Provider,
		Model:         dc.Model,
		Tools:         childTools,
		RecordMessage: recordMessage,
		Logger:        logger,
		SystemPrompt:  dc.SystemPrompt,
		WorkingDir:    ctx.WorkingDir,
		SessionID:     delegateID,
		Hooks:         dc.Hooks,
		SafeMode:      ctx.SafeMode,
		OnWorking: func(working bool) {
			subMgr.SetWorking(working)
			if !working {
				iteration++
				if iteration >= maxIterations {
					logger.Info("Sub-agent max iterations reached", "max", maxIterations)
				}
				// Cancel the child loop after each complete processing cycle.
				// The delegate queues exactly one task message, so the child
				// should exit after the first LLM response (success or error).
				cancelChild()
			}
		},
	})

	// Queue the task as a user message.
	loop.QueueUserMessage(llm.Message{
		Role:    llm.RoleUser,
		Content: args.Task,
	})

	logger.Info("Starting sub-agent", "max_iterations", maxIterations)

	// Run the child loop synchronously. It returns context.Canceled when we cancel it.
	err := loop.Go(childCtx)

	// Roll up sub-agent cost to parent session.
	subCost := subMgr.GetTotalCost()
	if dc.AddCost != nil {
		dc.AddCost(subCost)
	}

	// Notify parent that delegate completed.
	if dc.NotifyParent != nil {
		dc.NotifyParent(StreamResponse{
			DelegateEvent: &DelegateEvent{
				Type:       "completed",
				DelegateID: delegateID,
				Task:       args.Task,
				Cost:       subCost,
			},
		})
	}

	if err != nil && err != context.Canceled {
		logger.Error("Sub-agent failed", "error", err)
		return ToolOut{
			Content:    fmt.Sprintf("Sub-agent failed: %v", err),
			IsError:    true,
			DelegateID: delegateID,
		}
	}

	logger.Info("Sub-agent completed", "iterations", iteration)

	// Check if the child loop captured an error (e.g., provider failure).
	// This handles cases where the loop exits via context cancellation
	// but the underlying cause was an LLM error.
	if lastAssistantContent == "" && lastError != "" {
		return ToolOut{
			Content:    fmt.Sprintf("Sub-agent failed: %s", lastError),
			IsError:    true,
			DelegateID: delegateID,
		}
	}

	summary := lastAssistantContent
	if summary == "" {
		summary = "Sub-agent completed but produced no output."
	}

	return ToolOut{
		Content:    summary,
		DelegateID: delegateID,
	}
}

// filterOutTool returns a copy of the tools slice with the named tool removed.
func filterOutTool(tools []*AgentTool, name string) []*AgentTool {
	filtered := make([]*AgentTool, 0, len(tools))
	for _, t := range tools {
		if t.Function.Name != name {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// truncate shortens a string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

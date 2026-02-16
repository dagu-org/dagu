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

const (
	// delegateToolName is the name of the delegate tool.
	delegateToolName = "delegate"

	// maxConcurrentDelegates is the maximum number of sub-agents that can run in parallel.
	maxConcurrentDelegates = 10

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
				Description: "Spawn a sub-agent for a focused sub-task. The sub-agent works independently with the same tools and returns a summary. Use when a task benefits from dedicated, parallel execution. You can delegate multiple tasks simultaneously.",
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

	// Create sub-session in the store.
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

	// Filter out the delegate tool from child tools to prevent recursion.
	childTools := filterOutTool(dc.Tools, delegateToolName)

	// Create a cancellable context for the child loop.
	childCtx, cancelChild := context.WithCancel(ctx.Context)
	defer cancelChild()

	// Track last assistant content for summary extraction.
	var lastAssistantContent string
	iteration := 0

	logger := dc.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("delegate_id", delegateID, "task", truncate(args.Task, 100))

	// Build RecordMessage that persists to sub-session and captures last assistant content.
	var recordMessage MessageRecordFunc
	if dc.SessionStore != nil {
		recordMessage = func(msgCtx context.Context, msg Message) error {
			msg.SessionID = delegateID
			if msg.ID == "" {
				msg.ID = uuid.New().String()
			}
			if msg.Type == MessageTypeAssistant && msg.Content != "" {
				lastAssistantContent = msg.Content
			}
			return dc.SessionStore.AddMessage(msgCtx, delegateID, &msg)
		}
	} else {
		recordMessage = func(_ context.Context, msg Message) error {
			if msg.Type == MessageTypeAssistant && msg.Content != "" {
				lastAssistantContent = msg.Content
			}
			return nil
		}
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
			if !working {
				iteration++
				if iteration >= maxIterations {
					logger.Info("Sub-agent max iterations reached", "max", maxIterations)
					cancelChild()
				}
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
	if err != nil && err != context.Canceled {
		logger.Error("Sub-agent failed", "error", err)
		return ToolOut{
			Content:    fmt.Sprintf("Sub-agent failed: %v", err),
			IsError:    true,
			DelegateID: delegateID,
		}
	}

	logger.Info("Sub-agent completed", "iterations", iteration)

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

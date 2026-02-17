package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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

// delegateTask is a single sub-task within a batched delegate call.
type delegateTask struct {
	Task          string `json:"task"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

// delegateInput is the parsed input for the delegate tool.
type delegateInput struct {
	Tasks []delegateTask `json:"tasks"`
}

// singleDelegateResult holds the output of one sub-agent execution.
type singleDelegateResult struct {
	DelegateID string
	Content    string
	IsError    bool
	Cost       float64
}

// NewDelegateTool creates a delegate tool for spawning sub-agent loops.
func NewDelegateTool() *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name: delegateToolName,
				Description: fmt.Sprintf(
					"Spawn sub-agents for focused sub-tasks. Each sub-agent works independently "+
						"with the same tools and returns a summary. All tasks run in parallel. "+
						"Maximum %d tasks per call.",
					maxConcurrentDelegates,
				),
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tasks": map[string]any{
							"type": "array",
							"items": map[string]any{
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
							"maxItems":    maxConcurrentDelegates,
							"description": "List of sub-tasks to delegate to sub-agents in parallel",
						},
					},
					"required": []string{"tasks"},
				},
			},
		},
		Run: delegateRun,
	}
}

// delegateRun executes the delegate tool by spawning sub-agent loops in parallel.
func delegateRun(ctx ToolContext, input json.RawMessage) ToolOut {
	if ctx.Delegate == nil {
		return toolError("Delegate capability is not available in this context")
	}

	var args delegateInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Invalid input: %v", err)
	}

	if len(args.Tasks) == 0 {
		return toolError("At least one task is required")
	}

	// Cap at maxConcurrentDelegates.
	tasks := args.Tasks
	if len(tasks) > maxConcurrentDelegates {
		tasks = tasks[:maxConcurrentDelegates]
	}

	// Run all tasks in parallel.
	results := make([]singleDelegateResult, len(tasks))
	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t delegateTask) {
			defer wg.Done()
			results[idx] = runSingleDelegate(ctx, t)
		}(i, task)
	}
	wg.Wait()

	// Aggregate results.
	var delegateIDs []string
	var summaries []string
	allFailed := true

	for i, r := range results {
		delegateIDs = append(delegateIDs, r.DelegateID)
		prefix := fmt.Sprintf("[%d] %s", i+1, truncate(tasks[i].Task, 60))
		if r.IsError {
			summaries = append(summaries, fmt.Sprintf("%s: ERROR: %s", prefix, r.Content))
		} else {
			allFailed = false
			summaries = append(summaries, fmt.Sprintf("%s: %s", prefix, r.Content))
		}
	}

	return ToolOut{
		Content:     strings.Join(summaries, "\n\n"),
		IsError:     allFailed,
		DelegateIDs: delegateIDs,
	}
}

// runSingleDelegate executes one sub-agent for a single task.
func runSingleDelegate(ctx ToolContext, task delegateTask) singleDelegateResult {
	dc := ctx.Delegate
	delegateID := uuid.New().String()

	maxIterations := defaultDelegateMaxIterations
	if task.MaxIterations > 0 {
		maxIterations = task.MaxIterations
	}

	logger := dc.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("delegate_id", delegateID, "task", truncate(task.Task, 100))

	// Persist sub-session in the store.
	if dc.SessionStore != nil {
		now := time.Now()
		subSession := &Session{
			ID:              delegateID,
			UserID:          dc.UserID,
			ParentSessionID: dc.ParentID,
			DelegateTask:    task.Task,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := dc.SessionStore.CreateSession(ctx.Context, subSession); err != nil {
			return singleDelegateResult{
				DelegateID: delegateID,
				Content:    fmt.Sprintf("Failed to create sub-session: %v", err),
				IsError:    true,
			}
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
		DelegateTask:    task.Task,
	})

	// Register sub-session for SSE streaming.
	if dc.RegisterSubSession != nil {
		dc.RegisterSubSession(delegateID, subMgr)
	}

	// Set working state before notifying parent so the SSE snapshot has working=true.
	subMgr.SetWorking(true)

	// Notify parent that delegate started.
	if dc.NotifyParent != nil {
		dc.NotifyParent(StreamResponse{
			DelegateEvent: &DelegateEvent{
				Type:       "started",
				DelegateID: delegateID,
				Task:       task.Task,
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
				cancelChild()
			}
		},
	})

	// Queue the task as a user message.
	loop.QueueUserMessage(llm.Message{
		Role:    llm.RoleUser,
		Content: task.Task,
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
				Task:       task.Task,
				Cost:       subCost,
			},
		})
	}

	if err != nil && err != context.Canceled {
		logger.Error("Sub-agent failed", "error", err)
		return singleDelegateResult{
			DelegateID: delegateID,
			Content:    fmt.Sprintf("Sub-agent failed: %v", err),
			IsError:    true,
			Cost:       subCost,
		}
	}

	logger.Info("Sub-agent completed", "iterations", iteration)

	// Check if the child loop captured an error (e.g., provider failure).
	if lastAssistantContent == "" && lastError != "" {
		return singleDelegateResult{
			DelegateID: delegateID,
			Content:    fmt.Sprintf("Sub-agent failed: %s", lastError),
			IsError:    true,
			Cost:       subCost,
		}
	}

	summary := lastAssistantContent
	if summary == "" {
		summary = "Sub-agent completed but produced no output."
	}

	return singleDelegateResult{
		DelegateID: delegateID,
		Content:    summary,
		Cost:       subCost,
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

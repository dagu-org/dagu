// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"encoding/json"
	"fmt"

	"github.com/dagucloud/dagu/internal/llm"
)

const (
	listAutomataTasksToolName = "list_automata_tasks"
	listAllowedDAGsToolName   = "list_allowed_dags"
	runAllowedDAGToolName     = "run_allowed_dag"
	retryAutomataRunTool      = "retry_automata_run"
	setAutomataTaskDoneTool   = "set_automata_task_done"
	requestHumanInputTool     = "request_human_input"
	finishAutomataTool        = "finish_automata"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           listAutomataTasksToolName,
		Label:          "List Automata Tasks",
		Description:    "List the task list items for this Automata",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.AutomataRuntime == nil {
				return nil
			}
			return newListAutomataTasksTool(cfg.AutomataRuntime)
		},
	})
	RegisterTool(ToolRegistration{
		Name:           listAllowedDAGsToolName,
		Label:          "List Allowed DAGs",
		Description:    "List DAGs that this Automata is allowed to execute",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.AutomataRuntime == nil {
				return nil
			}
			return newListAllowedDAGsTool(cfg.AutomataRuntime)
		},
	})
	RegisterTool(ToolRegistration{
		Name:           runAllowedDAGToolName,
		Label:          "Run Allowed DAG",
		Description:    "Run an allowlisted DAG and pause the current Automata turn",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.AutomataRuntime == nil {
				return nil
			}
			return newRunAllowedDAGTool(cfg.AutomataRuntime)
		},
	})
	RegisterTool(ToolRegistration{
		Name:           retryAutomataRunTool,
		Label:          "Retry Automata Run",
		Description:    "Retry the last Automata-owned child DAG run",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.AutomataRuntime == nil {
				return nil
			}
			return newRetryAutomataRunTool(cfg.AutomataRuntime)
		},
	})
	RegisterTool(ToolRegistration{
		Name:           setAutomataTaskDoneTool,
		Label:          "Set Automata Task Done",
		Description:    "Mark an existing Automata task list item done or open",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.AutomataRuntime == nil {
				return nil
			}
			return newSetAutomataTaskDoneTool(cfg.AutomataRuntime)
		},
	})
	RegisterTool(ToolRegistration{
		Name:           requestHumanInputTool,
		Label:          "Request Human Input",
		Description:    "Pause this Automata and wait for a human response",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.AutomataRuntime == nil {
				return nil
			}
			return newRequestHumanInputTool(cfg.AutomataRuntime)
		},
	})
	RegisterTool(ToolRegistration{
		Name:           finishAutomataTool,
		Label:          "Finish Automata",
		Description:    "Mark this Automata as finished and pause the current turn",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.AutomataRuntime == nil {
				return nil
			}
			return newFinishAutomataTool(cfg.AutomataRuntime)
		},
	})
}

func newListAutomataTasksTool(runtime AutomataRuntime) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        listAutomataTasksToolName,
				Description: "Return the ordered task list items for this Automata.",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		Run: func(ctx ToolContext, _ json.RawMessage) ToolOut {
			items, err := runtime.ListTasks(ctx.Context)
			if err != nil {
				return toolError("failed to list automata tasks: %v", err)
			}
			body, err := json.MarshalIndent(items, "", "  ")
			if err != nil {
				return toolError("failed to format automata tasks: %v", err)
			}
			return ToolOut{Content: string(body)}
		},
	}
}

func newListAllowedDAGsTool(runtime AutomataRuntime) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        listAllowedDAGsToolName,
				Description: "Return the DAGs this Automata is allowed to execute, including descriptions and tags when available.",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		Run: func(ctx ToolContext, _ json.RawMessage) ToolOut {
			items, err := runtime.ListAllowedDAGs(ctx.Context)
			if err != nil {
				return toolError("failed to list allowed DAGs: %v", err)
			}
			body, err := json.MarshalIndent(items, "", "  ")
			if err != nil {
				return toolError("failed to format allowed DAGs: %v", err)
			}
			return ToolOut{Content: string(body)}
		},
	}
}

type runAllowedDAGInput struct {
	DAGName string `json:"dag_name"`
	Params  string `json:"params,omitempty"`
}

func newRunAllowedDAGTool(runtime AutomataRuntime) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        runAllowedDAGToolName,
				Description: "Launch one allowlisted DAG as the next unit of work, then pause until that DAG run changes state.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"dag_name": map[string]any{
							"type":        "string",
							"description": "Name of the allowlisted DAG to execute.",
						},
						"params": map[string]any{
							"type":        "string",
							"description": "Optional CLI params string to pass to the DAG run.",
						},
					},
					"required": []string{"dag_name"},
				},
			},
		},
		Run: func(ctx ToolContext, input json.RawMessage) ToolOut {
			var args runAllowedDAGInput
			if err := json.Unmarshal(input, &args); err != nil {
				return toolError("invalid input: %v", err)
			}
			result, err := runtime.RunAllowedDAG(ctx.Context, AutomataRunDAGInput{
				DAGName: args.DAGName,
				Params:  args.Params,
			})
			if err != nil {
				return toolError("failed to run DAG %q: %v", args.DAGName, err)
			}
			return ToolOut{
				Content:       fmt.Sprintf("Started DAG %q with run ID %q.", result.DAGName, result.DAGRunID),
				InterruptTurn: true,
			}
		},
	}
}

func newRetryAutomataRunTool(runtime AutomataRuntime) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        retryAutomataRunTool,
				Description: "Retry the most recent child DAG run owned by this Automata, then pause until the new run changes state.",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		Run: func(ctx ToolContext, _ json.RawMessage) ToolOut {
			result, err := runtime.RetryCurrentRun(ctx.Context)
			if err != nil {
				return toolError("failed to retry Automata run: %v", err)
			}
			return ToolOut{
				Content:       fmt.Sprintf("Retried DAG %q with run ID %q.", result.DAGName, result.DAGRunID),
				InterruptTurn: true,
			}
		},
	}
}

type setAutomataTaskDoneInput struct {
	TaskID string `json:"task_id"`
	Done   bool   `json:"done"`
}

func newSetAutomataTaskDoneTool(runtime AutomataRuntime) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        setAutomataTaskDoneTool,
				Description: "Mark one existing task list item done or open for this Automata.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"task_id": map[string]any{
							"type":        "string",
							"description": "ID of the task to update.",
						},
						"done": map[string]any{
							"type":        "boolean",
							"description": "True to mark the task done, false to mark it open again.",
						},
					},
					"required": []string{"task_id", "done"},
				},
			},
		},
		Run: func(ctx ToolContext, input json.RawMessage) ToolOut {
			var args setAutomataTaskDoneInput
			if err := json.Unmarshal(input, &args); err != nil {
				return toolError("invalid input: %v", err)
			}
			if err := runtime.SetTaskDone(ctx.Context, args.TaskID, args.Done); err != nil {
				return toolError("failed to update task %q: %v", args.TaskID, err)
			}
			if args.Done {
				return ToolOut{Content: fmt.Sprintf("Task %q marked done.", args.TaskID)}
			}
			return ToolOut{Content: fmt.Sprintf("Task %q marked open.", args.TaskID)}
		},
	}
}

type requestHumanInputInput struct {
	Question            string             `json:"question"`
	Options             []UserPromptOption `json:"options,omitempty"`
	AllowFreeText       bool               `json:"allow_free_text,omitempty"`
	FreeTextPlaceholder string             `json:"free_text_placeholder,omitempty"`
}

func newRequestHumanInputTool(runtime AutomataRuntime) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        requestHumanInputTool,
				Description: "Pause this Automata and ask a human for input. Use this when the workflow is blocked on approval or clarification.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"question": map[string]any{
							"type":        "string",
							"description": "Question to present to the human.",
						},
						"options": map[string]any{
							"type":        "array",
							"description": "Optional predefined choices.",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":          map[string]any{"type": "string"},
									"label":       map[string]any{"type": "string"},
									"description": map[string]any{"type": "string"},
								},
								"required": []string{"id", "label"},
							},
						},
						"allow_free_text": map[string]any{
							"type":        "boolean",
							"description": "Whether the human may enter free text.",
						},
						"free_text_placeholder": map[string]any{
							"type":        "string",
							"description": "Optional placeholder for the free text field.",
						},
					},
					"required": []string{"question"},
				},
			},
		},
		Run: func(ctx ToolContext, input json.RawMessage) ToolOut {
			var args requestHumanInputInput
			if err := json.Unmarshal(input, &args); err != nil {
				return toolError("invalid input: %v", err)
			}
			if err := runtime.RequestHumanInput(ctx.Context, AutomataHumanPrompt{
				Question:            args.Question,
				Options:             args.Options,
				AllowFreeText:       args.AllowFreeText,
				FreeTextPlaceholder: args.FreeTextPlaceholder,
			}); err != nil {
				return toolError("failed to request human input: %v", err)
			}
			return ToolOut{
				Content:       "Automata is now waiting for human input.",
				InterruptTurn: true,
			}
		},
	}
}

type finishAutomataInput struct {
	Summary string `json:"summary"`
}

func newFinishAutomataTool(runtime AutomataRuntime) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        finishAutomataTool,
				Description: "Mark this Automata as finished once its goal is complete.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"summary": map[string]any{
							"type":        "string",
							"description": "Short completion summary for the Automata history.",
						},
					},
					"required": []string{"summary"},
				},
			},
		},
		Run: func(ctx ToolContext, input json.RawMessage) ToolOut {
			var args finishAutomataInput
			if err := json.Unmarshal(input, &args); err != nil {
				return toolError("invalid input: %v", err)
			}
			if err := runtime.Finish(ctx.Context, args.Summary); err != nil {
				return toolError("failed to finish Automata: %v", err)
			}
			return ToolOut{
				Content:       "Automata marked as finished.",
				InterruptTurn: true,
			}
		},
	}
}

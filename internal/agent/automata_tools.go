// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"encoding/json"
	"fmt"

	"github.com/dagu-org/dagu/internal/llm"
)

const (
	listAllowedDAGsToolName = "list_allowed_dags"
	runAllowedDAGToolName   = "run_allowed_dag"
	retryAutomataRunTool    = "retry_automata_run"
	setAutomataStageTool    = "set_automata_stage"
	requestHumanInputTool   = "request_human_input"
	finishAutomataTool      = "finish_automata"
)

func init() {
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
		Name:           setAutomataStageTool,
		Label:          "Set Automata Stage",
		Description:    "Update the current stage for this Automata",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.AutomataRuntime == nil {
				return nil
			}
			return newSetAutomataStageTool(cfg.AutomataRuntime)
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

type setAutomataStageInput struct {
	Stage string `json:"stage"`
	Note  string `json:"note,omitempty"`
}

func newSetAutomataStageTool(runtime AutomataRuntime) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        setAutomataStageTool,
				Description: "Update the Automata's current stage. The stage must be one of the declared stage names for this Automata.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"stage": map[string]any{
							"type":        "string",
							"description": "New stage name.",
						},
						"note": map[string]any{
							"type":        "string",
							"description": "Optional short reason for the stage transition.",
						},
					},
					"required": []string{"stage"},
				},
			},
		},
		Run: func(ctx ToolContext, input json.RawMessage) ToolOut {
			var args setAutomataStageInput
			if err := json.Unmarshal(input, &args); err != nil {
				return toolError("invalid input: %v", err)
			}
			if err := runtime.SetStage(ctx.Context, args.Stage, args.Note); err != nil {
				return toolError("failed to set stage %q: %v", args.Stage, err)
			}
			return ToolOut{Content: fmt.Sprintf("Automata stage set to %q.", args.Stage)}
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

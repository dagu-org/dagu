// Package agentstep provides an executor for agent-type workflow steps.
// It wraps the agent.Loop to run the Tsumugi AI agent as a single-shot
// step within a DAG workflow.
package agentstep

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"

	// Import all LLM providers to register them.
	_ "github.com/dagu-org/dagu/internal/llm/allproviders"
)

var _ executor.Executor = (*Executor)(nil)

func init() {
	executor.RegisterExecutor(
		core.ExecutorTypeAgent,
		newAgentExecutor,
		nil,
		core.ExecutorCapabilities{Agent: true},
	)
}

// Executor runs the agent loop as a workflow step.
type Executor struct {
	step   core.Step
	stdout io.Writer
	stderr io.Writer
}

func newAgentExecutor(_ context.Context, step core.Step) (executor.Executor, error) {
	return &Executor{step: step}, nil
}

func (e *Executor) SetStdout(w io.Writer) { e.stdout = w }
func (e *Executor) SetStderr(w io.Writer) { e.stderr = w }

func (e *Executor) Kill(_ os.Signal) error {
	return nil
}

func (e *Executor) Run(ctx context.Context) error {
	dagCtx := exec.GetContext(ctx)
	stderr := e.stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	stdout := e.stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	// Resolve agent configuration from exec context.
	configStore, ok := dagCtx.AgentConfigStore.(agent.ConfigStore)
	if !ok || configStore == nil {
		return fmt.Errorf("agent config store not available; ensure the agent feature is configured")
	}
	modelStore, ok := dagCtx.AgentModelStore.(agent.ModelStore)
	if !ok || modelStore == nil {
		return fmt.Errorf("agent model store not available; ensure models are configured in Agent Settings")
	}

	agentCfg, err := configStore.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load agent config: %w", err)
	}

	// Resolve global tool policy for tool filtering and bash enforcement.
	globalPolicy := agent.ResolveToolPolicy(agentCfg.ToolPolicy)

	// Resolve model ID: step override → global default.
	modelID := agentCfg.DefaultModelID
	stepCfg := e.step.Agent
	if stepCfg != nil && stepCfg.Model != "" {
		modelID = stepCfg.Model
	}
	if modelID == "" {
		return fmt.Errorf("no model configured; set a default model in Agent Settings or specify agent.model in the step")
	}

	modelCfg, err := modelStore.GetByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("failed to get model %q: %w", modelID, err)
	}

	// Create LLM provider.
	provider, err := agent.CreateLLMProvider(modelCfg.ToLLMConfig())
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Build tools filtered by global policy (exclude navigate and ask_user; add output tool).
	tools := buildTools(dagCtx, stepCfg, globalPolicy, stdout)

	// Generate system prompt.
	systemPrompt := buildSystemPrompt(dagCtx, stepCfg)

	// Resolve safe mode and max iterations.
	safeMode := true
	maxIterations := 50
	if stepCfg != nil {
		safeMode = stepCfg.SafeMode
		if stepCfg.MaxIterations > 0 {
			maxIterations = stepCfg.MaxIterations
		}
	}

	// Evaluate variable substitution in messages.
	var userMessages []llm.Message
	for _, msg := range e.step.Messages {
		content, evalErr := runtime.EvalString(ctx, msg.Content)
		if evalErr != nil {
			return fmt.Errorf("failed to evaluate message content: %w", evalErr)
		}
		userMessages = append(userMessages, llm.Message{
			Role:    llm.Role(msg.Role),
			Content: content,
		})
	}

	// Register bash policy enforcement hook.
	hooks := agent.NewHooks()
	hooks.OnBeforeToolExec(buildPolicyHook(globalPolicy))

	logf(stderr, "Starting (model: %s, tools: %d, safe_mode: %v, max_iterations: %d)",
		modelCfg.Name, len(tools), safeMode, maxIterations)

	// Create a cancellable context for stopping the loop after completion.
	loopCtx, cancelLoop := context.WithCancel(ctx)
	defer cancelLoop()

	iteration := 0

	loop := agent.NewLoop(agent.LoopConfig{
		Provider:     provider,
		Model:        modelCfg.Model,
		Tools:        tools,
		SystemPrompt: systemPrompt,
		SafeMode:     safeMode,
		Hooks:        hooks,
		Logger:       slog.Default(),
		RecordMessage: func(_ context.Context, msg agent.Message) error {
			logMessage(stderr, msg)
			return nil
		},
		OnWorking: func(working bool) {
			if !working {
				iteration++
				if iteration >= maxIterations {
					logf(stderr, "Max iterations reached (%d)", maxIterations)
					cancelLoop()
					return
				}
				// Agent finished processing a turn with no more tool calls.
				// Cancel to stop the polling loop.
				cancelLoop()
			}
		},
	})

	// Queue user messages.
	for _, msg := range userMessages {
		loop.QueueUserMessage(msg)
	}

	// Run the loop. It returns context.Canceled when we cancel it (expected).
	err = loop.Go(loopCtx)
	if err != nil && err != context.Canceled {
		logf(stderr, "Failed: %v", err)
		return err
	}

	logf(stderr, "Completed (%d iterations)", iteration)
	return nil
}

// buildTools creates the tool list for the agent step.
// Tools are filtered first by global policy, then by step-level config.
func buildTools(dagCtx exec.Context, stepCfg *core.AgentStepConfig, globalPolicy agent.ToolPolicyConfig, stdout io.Writer) []*agent.AgentTool {
	dagsDir := ""
	if dagCtx.DAG != nil {
		dagsDir = dagCtx.DAG.Location
	}

	// All available tools (excluding navigate and ask_user).
	allTools := map[string]*agent.AgentTool{
		"bash":        agent.NewBashTool(),
		"read":        agent.NewReadTool(),
		"patch":       agent.NewPatchTool(dagsDir),
		"think":       agent.NewThinkTool(),
		"read_schema": agent.NewReadSchemaTool(),
		"web_search":  agent.NewWebSearchTool(),
		"output":      agent.NewOutputTool(stdout),
	}

	// Remove tools disabled by global policy (output is step-only, always kept).
	for name := range allTools {
		if name != "output" && !agent.IsToolEnabledResolved(globalPolicy, name) {
			delete(allTools, name)
		}
	}

	// If step specifies enabled tools, filter to only those.
	if stepCfg != nil && stepCfg.Tools != nil && len(stepCfg.Tools.Enabled) > 0 {
		var tools []*agent.AgentTool
		for _, name := range stepCfg.Tools.Enabled {
			if tool, ok := allTools[name]; ok {
				tools = append(tools, tool)
			}
		}
		// Always include the output tool even if not explicitly listed.
		if agent.GetToolByName(tools, "output") == nil {
			tools = append(tools, allTools["output"])
		}
		return tools
	}

	// Default: all globally-enabled tools.
	var tools []*agent.AgentTool
	for _, tool := range allTools {
		tools = append(tools, tool)
	}
	return tools
}

// buildSystemPrompt generates the system prompt for the agent step.
func buildSystemPrompt(dagCtx exec.Context, stepCfg *core.AgentStepConfig) string {
	env := agent.EnvironmentInfo{}
	if dagCtx.DAG != nil {
		env.DAGsDir = dagCtx.DAG.Location
	}

	var currentDAG *agent.CurrentDAG
	if dagCtx.DAG != nil {
		currentDAG = &agent.CurrentDAG{
			Name: dagCtx.DAG.Name,
		}
	}

	prompt := agent.GenerateSystemPrompt(env, currentDAG, agent.MemoryContent{}, "")

	// Append instruction about the output tool.
	prompt += "\n\n## Output\n\nWhen you have completed your task, use the `output` tool to write your final result. " +
		"The content you provide to the output tool will be captured as the step's output variable for downstream steps.\n"

	// Append step-level prompt if specified.
	if stepCfg != nil && stepCfg.Prompt != "" {
		prompt += "\n## Additional Instructions\n\n" + stepCfg.Prompt + "\n"
	}

	return prompt
}

// buildPolicyHook returns a before-tool hook that enforces bash command policy.
func buildPolicyHook(policy agent.ToolPolicyConfig) agent.BeforeToolExecHookFunc {
	return func(_ context.Context, info agent.ToolExecInfo) error {
		if !agent.IsBashToolName(info.ToolName) {
			return nil
		}
		decision, err := agent.EvaluateBashPolicyResolved(policy, info.Input)
		if err != nil {
			return fmt.Errorf("bash policy evaluation failed: %w", err)
		}
		if !decision.Allowed {
			return fmt.Errorf("bash command denied by policy: %s", decision.Reason)
		}
		return nil
	}
}

// logf writes a formatted log line to stderr with [agent] prefix.
func logf(w io.Writer, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(w, "[agent] %s\n", msg)
}

// logMessage writes a structured log entry for an agent message.
func logMessage(w io.Writer, msg agent.Message) {
	switch msg.Type {
	case agent.MessageTypeAssistant:
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				args := tc.Function.Arguments
				if len(args) > 200 {
					args = args[:200] + "..."
				}
				logf(w, "Tool call: %s %s", tc.Function.Name, args)
			}
		}
		if msg.Content != "" {
			content := msg.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			logf(w, "Assistant: %s", strings.ReplaceAll(content, "\n", " "))
		}

	case agent.MessageTypeUser:
		if len(msg.ToolResults) > 0 {
			for _, tr := range msg.ToolResults {
				status := "success"
				if tr.IsError {
					status = "error"
				}
				logf(w, "Tool result: [%s, %d chars]", status, len(tr.Content))
			}
		}

	case agent.MessageTypeError:
		logf(w, "Error: %s", msg.Content)
	}
}

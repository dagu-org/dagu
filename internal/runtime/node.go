package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"syscall"

	"github.com/dagu-org/dagu/internal/cmn/collections"
	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/signal"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

// systemVarPrefix is the prefix for temporary variables used internally by Dagu
// to avoid conflicts with user-defined variables.
const systemVarPrefix = "DAGU_"

// Node is a node in a DAG. It executes a command.
type Node struct {
	Data
	outputs OutputCoordinator

	id           int
	mu           sync.RWMutex
	cmd          executor.Executor
	done         atomic.Bool
	retryPolicy  RetryPolicy
	cmdEvaluated atomic.Bool
}

func NewNode(step core.Step, state NodeState) *Node {
	return &Node{
		Data: newSafeData(NodeData{Step: step, State: state}),
	}
}

func NodeWithData(data NodeData) *Node {
	return &Node{
		Data: newSafeData(data),
	}
}

func (n *Node) NodeData() NodeData {
	return n.Data.Data()
}

// OutputVariablesMap returns output variables as key->value map.
// This is used to build the EnvScope chain with predecessor outputs.
func (n *Node) OutputVariablesMap() map[string]string {
	result := make(map[string]string)
	state := n.State()
	if state.OutputVariables == nil {
		return result
	}
	state.OutputVariables.Range(func(key, value any) bool {
		// Values are stored as "key=value" format
		if strVal, ok := value.(string); ok {
			if _, v, found := strings.Cut(strVal, "="); found {
				result[key.(string)] = v
			}
		}
		return true
	})
	return result
}

func (n *Node) ID() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.id
}

func (n *Node) StdoutFile() string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.outputs.StdoutFile()
}

func (n *Node) ShouldMarkSuccess(ctx context.Context) bool {
	if !n.ShouldContinue(ctx) {
		return false
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.ContinueOn().MarkSuccess
}

func (n *Node) ShouldContinue(ctx context.Context) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	continueOn := n.ContinueOn()

	s := n.Status()
	switch s {
	case core.NodeSucceeded:
		return true

	case core.NodeFailed:
		if continueOn.Failure {
			return true
		}

	case core.NodeAborted:
		return false

	case core.NodeSkipped:
		if continueOn.Skipped {
			return true
		}
		return false

	case core.NodePartiallySucceeded:
		// Partial success is treated like success for continue on
		return true

	case core.NodeRejected:
		// Rejected nodes should not allow dependents to continue
		return false

	case core.NodeNotStarted, core.NodeRunning, core.NodeWaiting:
		// Unexpected state
		logger.Error(ctx, "Unexpected node status",
			tag.Status(s.String()),
		)
		return false

	}

	cacheKey := systemVarPrefix + "CONTINUE_ON." + n.Name()
	if v, ok := n.getBoolVariable(cacheKey); ok {
		return v
	}

	if n.MatchExitCode(continueOn.ExitCode) {
		n.setBoolVariable(cacheKey, true)
		return true
	}

	if len(continueOn.Output) > 0 {
		ok, err := n.LogContainsPattern(ctx, continueOn.Output)
		if err != nil {
			logger.Error(ctx, "Failed to check log for pattern", tag.Error(err))
			return false
		}
		if ok {
			n.setBoolVariable(cacheKey, true)
			return true
		}
	}

	n.setBoolVariable(cacheKey, false)
	return false
}

func (n *Node) Execute(ctx context.Context) error {
	ctx, cancel, stepTimeout := n.setupContextWithTimeout(ctx)
	defer cancel()

	cmd, err := n.setupExecutor(ctx)
	if err != nil {
		n.SetError(fmt.Errorf("failed to setup executor: %w", err))
		return err
	}

	// Ensure executor cleanup happens (releases connections, etc.)
	defer func() {
		if closeErr := executor.CloseExecutor(cmd); closeErr != nil {
			logger.Warn(ctx, "Failed to close executor",
				tag.Step(n.Name()),
				tag.Error(closeErr))
		}
	}()

	// Check if executor supports chat message handling
	chatHandler, _ := cmd.(executor.ChatMessageHandler)

	// Set chat context from prior steps
	if chatHandler != nil {
		if messages := n.GetChatMessages(); len(messages) > 0 {
			chatHandler.SetContext(messages)
		}
	}

	flusher := n.startOutputFlusher()
	defer func() {
		n.stopOutputFlusher(flusher)
	}()

	exitCode, err := n.runCommand(ctx, cmd, stepTimeout)
	n.SetError(err)
	n.SetExitCode(exitCode)

	// Capture chat messages after execution
	if chatHandler != nil {
		n.SetChatMessages(chatHandler.GetMessages())
	}

	// Capture sub-runs from executors that spawn sub-DAGs (like chat with tools)
	if subRunProvider, ok := cmd.(executor.SubRunProvider); ok {
		// For repeated executions, accumulate previous sub-runs before setting new ones
		if n.IsRepeated() && len(n.State().SubRuns) > 0 {
			n.AddSubRunsRepeated(n.State().SubRuns...)
		}

		subRuns := subRunProvider.GetSubRuns()
		// Convert exec.SubDAGRun to runtime.SubDAGRun
		runtimeSubRuns := make([]SubDAGRun, len(subRuns))
		for i, sr := range subRuns {
			runtimeSubRuns[i] = SubDAGRun(sr)
		}
		n.SetSubRuns(runtimeSubRuns) // May be empty if no tool calls this iteration
	}

	// Capture tool definitions from chat executors for UI visibility
	if toolDefProvider, ok := cmd.(executor.ToolDefinitionProvider); ok {
		toolDefs := toolDefProvider.GetToolDefinitions()
		if len(toolDefs) > 0 {
			n.SetToolDefinitions(toolDefs)
		}
	}

	if err := n.captureOutput(ctx); err != nil {
		return err
	}

	if err := n.determineNodeStatus(cmd); err != nil {
		return err
	}

	return n.Error()
}

// setupContextWithTimeout configures the execution context with step-level timeout if specified.
func (n *Node) setupContextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc, time.Duration) {
	step := n.Step()
	var stepTimeout time.Duration

	if step.Timeout > 0 {
		stepTimeout = step.Timeout
		ctx, cancel := context.WithTimeout(ctx, stepTimeout)
		logger.Info(ctx, "Step execution started with timeout",
			tag.Timeout(stepTimeout),
		)
		return ctx, cancel, stepTimeout
	}

	ctx, cancel := context.WithCancel(ctx)
	return ctx, cancel, 0
}

// flusherControl coordinates shutdown of the output flusher goroutine.
type flusherControl struct {
	done     chan struct{} // Signals the flusher to stop
	finished chan struct{} // Closed when the flusher exits
}

// startOutputFlusher starts a goroutine that periodically flushes output buffers.
// It returns a flusherControl that must be passed to stopOutputFlusher to ensure
// proper synchronization when stopping.
func (n *Node) startOutputFlusher() *flusherControl {
	ctrl := &flusherControl{
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}
	go func() {
		defer close(ctrl.finished) // Signal when goroutine exits
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctrl.done:
				return
			case <-ticker.C:
				_ = n.outputs.flushWriters()
			}
		}
	}()
	return ctrl
}

// stopOutputFlusher stops the flusher goroutine, waits for it to exit,
// then performs a final flush.
func (n *Node) stopOutputFlusher(ctrl *flusherControl) {
	close(ctrl.done)
	<-ctrl.finished
	_ = n.outputs.flushWriters()
}

// runCommand executes the command and handles errors, timeouts, and exit codes.
func (n *Node) runCommand(ctx context.Context, cmd executor.Executor, stepTimeout time.Duration) (int, error) {
	startTime := time.Now()
	err := cmd.Run(ctx)

	if err != nil {
		elapsed := time.Since(startTime)
		step := n.Step()

		// Check if this is a timeout error
		if stepTimeout > 0 && (ctx.Err() == context.DeadlineExceeded || elapsed >= stepTimeout) {
			return n.handleTimeout(ctx, step, stepTimeout, elapsed)
		}

		return n.handleCommandError(cmd, err)
	}

	return 0, nil
}

// handleTimeout handles step-level timeout errors.
func (n *Node) handleTimeout(ctx context.Context, _ core.Step, stepTimeout, elapsed time.Duration) (int, error) {
	timeoutErr := fmt.Errorf("step timed out after %v (timeout: %v): %w",
		elapsed.Truncate(time.Millisecond), stepTimeout, context.DeadlineExceeded)
	logger.Error(ctx, "Step execution timed out",
		tag.Timeout(stepTimeout),
		tag.Duration(elapsed),
	)
	n.SetError(timeoutErr)
	n.SetStatus(core.NodeFailed)
	return 124, timeoutErr // Standard timeout exit code
}

// handleCommandError determines the exit code from a command execution error.
func (n *Node) handleCommandError(cmd executor.Executor, err error) (int, error) {
	n.SetError(err)

	// Try to get exit code from ExitCoder interface
	if exitCoder, ok := cmd.(executor.ExitCoder); ok {
		return exitCoder.ExitCode(), err
	}

	if code, found := exitCodeFromError(err); found {
		return code, err
	}

	// Default error exit code
	return 1, err
}

// captureOutput captures and stores the command output to a variable if configured.
func (n *Node) captureOutput(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	output := n.Step().Output
	if output == "" {
		return nil
	}

	value, err := n.outputs.capturedOutput(ctx)
	if err != nil {
		return fmt.Errorf("failed to capture output: %w", err)
	}
	n.setVariable(output, value)
	return nil
}

// determineNodeStatus uses the executor to determine the final node status if supported.
func (n *Node) determineNodeStatus(cmd executor.Executor) error {
	statusDeterminer, ok := cmd.(executor.NodeStatusDeterminer)
	if !ok {
		return nil
	}

	nodeStatus, err := statusDeterminer.DetermineNodeStatus()
	if err != nil {
		return err
	}
	n.SetStatus(nodeStatus)
	return nil
}

// clearVariable removes a variable from the Node's internal state.
// Variables are scoped to the Node to avoid race conditions when
// multiple DAG steps or concurrent tasks run simultaneously.
func (n *Node) clearVariable(key string) {
	n.ClearVariable(key)
}

func (n *Node) setupExecutor(ctx context.Context) (executor.Executor, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Clear the cache
	n.clearVariable(systemVarPrefix + "CONTINUE_ON." + n.Name())

	// Reset the state
	n.ResetError()

	// Reset the done flag
	n.done.Store(false)

	// Evaluate the command and args if not already evaluated
	if err := n.evaluateCommandArgs(ctx); err != nil {
		return nil, err
	}

	// Evaluate the executor config if set
	execConfig := n.Step().ExecutorConfig
	cfg, err := EvalObject(ctx, n.Step().ExecutorConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to eval executor config: %w", err)
	}
	execConfig.Config = cfg
	n.SetExecutorConfig(execConfig)

	// Evaluate the sub DAG if set
	if child := n.Step().SubDAG; child != nil {
		dagName, err := EvalString(ctx, child.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to eval sub DAG name: %w", err)
		}
		copy := *child
		copy.Name = dagName
		n.SetSubDAG(copy)
	}

	// Evaluate script if set
	if script := n.Step().Script; script != "" {
		var opts []eval.Option
		if n.Step().ExecutorConfig.IsCommand() {
			opts = append(opts, eval.OnlyReplaceVars())
		}
		script, err := EvalString(ctx, script, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to eval script: %w", err)
		}
		n.SetScript(script)
	}

	// Create the executor
	cmd, err := executor.NewExecutor(ctx, n.Step())
	if err != nil {
		return nil, err
	}
	n.cmd = cmd

	if err := n.outputs.setupExecutorIO(ctx, cmd, n.NodeData()); err != nil {
		return nil, fmt.Errorf("failed to setup executor IO: %w", err)
	}

	// Handle sub DAG execution
	if subDAG := n.Step().SubDAG; subDAG != nil {
		subRuns, err := n.BuildSubDAGRuns(ctx, subDAG)
		if err != nil {
			return nil, err
		}
		n.SetSubRuns(subRuns)

		// Setup the executor with sub DAG run information
		if n.Step().Parallel == nil {
			// Single sub DAG execution
			exec, ok := cmd.(executor.DAGExecutor)
			if !ok {
				return nil, fmt.Errorf("executor %T does not support sub DAG execution", cmd)
			}
			exec.SetParams(executor.RunParams{
				RunID:  subRuns[0].DAGRunID,
				Params: subRuns[0].Params,
			})
		} else {
			// Parallel sub DAG execution
			exec, ok := cmd.(executor.ParallelExecutor)
			if !ok {
				return nil, fmt.Errorf("executor %T does not support parallel execution", cmd)
			}
			// Convert SubDAGRun to executor.RunParams
			var runParamsList []executor.RunParams
			for _, subRun := range subRuns {
				runParamsList = append(runParamsList, executor.RunParams{
					RunID:  subRun.DAGRunID,
					Params: subRun.Params,
				})
			}
			exec.SetParamsList(runParamsList)
		}
	}

	return cmd, nil
}

// evaluateCommandArgs evaluates the command and arguments of the node.
func (n *Node) evaluateCommandArgs(ctx context.Context) error {
	if n.cmdEvaluated.Load() {
		return nil
	}

	// Get eval options from executor capabilities
	evalOptions := n.Step().EvalOptions(ctx)

	step := n.Step()

	if len(step.Commands) > 0 {
		commands := make([]core.CommandEntry, len(step.Commands))
		for i, cmdEntry := range step.Commands {
			args := make([]string, len(cmdEntry.Args))
			for j, arg := range cmdEntry.Args {
				value, err := EvalString(ctx, arg, evalOptions...)
				if err != nil {
					return fmt.Errorf("failed to eval command args: %w", err)
				}
				args[j] = value
			}

			// Evaluate CmdWithArgs if present
			cmdWithArgs := cmdEntry.CmdWithArgs
			if cmdWithArgs != "" {
				evaluated, err := EvalString(ctx, cmdWithArgs, evalOptions...)
				if err != nil {
					return fmt.Errorf("failed to eval command with args: %w", err)
				}
				cmdWithArgs = evaluated
			}

			commands[i] = core.CommandEntry{
				Command:     cmdEntry.Command,
				Args:        args,
				CmdWithArgs: cmdWithArgs,
			}
		}
		step.Commands = commands

		n.SetStep(step)
		n.cmdEvaluated.Store(true)
		return nil
	}

	// No commands to evaluate
	n.cmdEvaluated.Store(true)
	return nil
}

func (n *Node) Signal(ctx context.Context, sig os.Signal, allowOverride bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	s := n.Status()
	if s == core.NodeRunning && n.cmd != nil {
		sigsig := sig
		if allowOverride && n.SignalOnStop() != "" {
			sigsig = syscall.Signal(signal.GetSignalNum(n.SignalOnStop()))
		}
		logger.Info(ctx, "Sending signal",
			tag.Signal(sigsig.String()),
			tag.Step(n.Name()),
		)
		if err := n.cmd.Kill(sigsig); err != nil {
			logger.Error(ctx, "Failed to send signal",
				tag.Error(err),
				tag.Step(n.Name()),
			)
		}
	}

	if signal.IsTerminationSignalOS(sig) {
		if s == core.NodeRunning {
			n.SetStatus(core.NodeAborted)
		}
	}
}

func (n *Node) Cancel() {
	n.mu.Lock()
	defer n.mu.Unlock()
	s := n.Status()
	if s == core.NodeRunning {
		n.SetStatus(core.NodeAborted)
	}
}

func (n *Node) SetupEnv(ctx context.Context) context.Context {
	n.mu.RLock()
	defer n.mu.RUnlock()
	env := GetEnv(ctx)
	env.Scope = env.Scope.WithEntry(
		exec.EnvKeyDAGRunStepStdoutFile, n.GetStdout(), eval.EnvSourceStepEnv,
	).WithEntry(
		exec.EnvKeyDAGRunStepStderrFile, n.GetStderr(), eval.EnvSourceStepEnv,
	)
	ctx = logger.WithValues(ctx, tag.Step(n.Name()))
	return WithEnv(ctx, env)
}

func (n *Node) Prepare(ctx context.Context, logDir string, dagRunID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Set the log file path
	startedAt := time.Now()
	safeName := fileutil.SafeName(n.Name())
	timestamp := startedAt.Format("20060102.150405.000")
	postfix := stringutil.TruncString(dagRunID, 8)
	logFilename := fmt.Sprintf("%s.%s.%s", safeName, timestamp, postfix)
	if !fileutil.FileExists(logDir) {
		if err := os.MkdirAll(logDir, 0750); err != nil {
			return fmt.Errorf("failed to create log directory %q: %w", logDir, err)
		}
	}

	logFile := filepath.Join(logDir, logFilename)
	if err := n.Setup(ctx, logFile, startedAt); err != nil {
		return fmt.Errorf("failed to setup node data: %w", err)
	}
	if err := n.outputs.setup(ctx, n.NodeData()); err != nil {
		return fmt.Errorf("failed to setup outputs: %w", err)
	}
	if err := n.setupRetryPolicy(ctx); err != nil {
		return fmt.Errorf("failed to setup retry policy: %w", err)
	}
	return nil
}

func (n *Node) Teardown() error {
	// Atomically mark as done to prevent concurrent teardown
	if !n.done.CompareAndSwap(false, true) {
		return nil
	}

	var lastErr error
	if err := n.outputs.closeResources(); err != nil {
		lastErr = err
	}

	if lastErr != nil {
		n.SetError(lastErr)
	}

	return lastErr
}

// LogContainsPattern checks if any of the given patterns exist in the node's log file.
// If a pattern starts with "regexp:", it will be treated as a regular expression.
// Returns false if no log file exists or no pattern is found.
// Returns error if there are issues reading the file or invalid regex pattern.
func (n *Node) LogContainsPattern(ctx context.Context, patterns []string) (bool, error) {
	if len(patterns) == 0 {
		return false, nil
	}

	// Get the log filename and check if it exists
	logFilename := n.outputs.StdoutFile()
	if logFilename == "" {
		return false, nil
	}

	// Open the log file
	file, err := os.Open(logFilename) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// Get maxOutputSize from DAG configuration
	var maxOutputSize = 1024 * 1024 // Default 1MB
	if rCtx := GetDAGContext(ctx); rCtx.DAG != nil && rCtx.DAG.MaxOutputSize > 0 {
		maxOutputSize = rCtx.DAG.MaxOutputSize
	}

	// Create scanner with default buffer, but configure max size based on DAG config
	scanner := bufio.NewScanner(file)
	// Set scanner buffer to handle lines up to maxOutputSize
	// Start with default 64KB initial buffer, but allow growth up to maxOutputSize
	scanner.Buffer(make([]byte, 0, 64*1024), maxOutputSize)

	// Use the logLock to prevent concurrent file operations
	n.outputs.lock()
	defer n.outputs.unlock()

	if stringutil.MatchPatternScanner(ctx, scanner, patterns) {
		return true, nil
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("failed to read log file: %w", err)
	}

	return false, nil
}

var (
	nextNodeID = 1
	nextNodeMu sync.Mutex
)

func getNextNodeID() int {
	nextNodeMu.Lock()
	defer nextNodeMu.Unlock()
	v := nextNodeID
	nextNodeID++
	return v
}

func (n *Node) Init() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.id != 0 {
		return
	}
	n.id = getNextNodeID()
}

// BuildSubDAGRuns constructs the sub DAG runs based on parallel configuration
func (n *Node) BuildSubDAGRuns(ctx context.Context, subDAG *core.SubDAG) ([]SubDAGRun, error) {
	parallel := n.Step().Parallel

	// Single sub DAG execution (non-parallel)
	if parallel == nil {
		params, err := EvalString(ctx, subDAG.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to eval sub dag params: %w", err)
		}
		repeated := n.IsRepeated()
		if repeated && len(n.State().SubRuns) > 0 {
			n.AddSubRunsRepeated(n.State().SubRuns[0])
		}
		dagRunID := GenerateSubDAGRunID(ctx, params, repeated)
		return []SubDAGRun{{
			DAGRunID: dagRunID,
			Params:   params,
		}}, nil
	}

	// Parallel execution
	var items []any

	// Handle variable reference
	if parallel.Variable != "" {
		value, err := EvalString(ctx, parallel.Variable)
		if err != nil {
			return nil, fmt.Errorf("failed to eval parallel variable %q: %w", parallel.Variable, err)
		}

		// Parse the value using smart separator detection
		parsedItems, err := stringutil.ParseSeparatedValues(value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse parallel variable %q: %w", parallel.Variable, err)
		}
		for _, item := range parsedItems {
			items = append(items, item)
		}
	} else if len(parallel.Items) > 0 {
		// Handle static items
		for _, item := range parallel.Items {
			if item.Value != "" {
				value, err := EvalString(ctx, item.Value)
				if err != nil {
					return nil, fmt.Errorf("failed to eval parallel item value %q: %w", item.Value, err)
				}
				items = append(items, value)
			} else if len(item.Params) > 0 {
				// evaluate each value in Params
				m := make(collections.DeterministicMap)
				for key, value := range item.Params {
					evaluatedValue, err := EvalString(ctx, value)
					if err != nil {
						return nil, fmt.Errorf("failed to eval parallel item param %q: %w", key, err)
					}
					m[key] = evaluatedValue
				}
				// Convert to JSON string
				paramData, err := json.Marshal(m)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal params: %w", err)
				}
				items = append(items, json.RawMessage(paramData))
			}
		}
	}

	// Validate we have items
	if len(items) == 0 {
		return nil, fmt.Errorf("parallel execution requires at least one item")
	}

	// Validate maximum number of items
	const maxParallelItems = 1000
	if len(items) > maxParallelItems {
		return nil, fmt.Errorf("parallel execution exceeds maximum limit: %d items (max: %d)", len(items), maxParallelItems)
	}

	// Build sub runs with deduplication
	subRunMap := make(map[string]SubDAGRun)
	repeated := n.IsRepeated()

	if repeated {
		n.AddSubRunsRepeated(n.State().SubRuns...)
	}

	for i, item := range items {
		param, err := n.ItemToParam(item)
		if err != nil {
			return nil, fmt.Errorf("failed to process item %d: %w", i, err)
		}

		// Merge the item param with the step's params if they exist
		finalParams := param
		if subDAG.Params != "" {
			// Create variables map with ITEM set to the current item value
			variables := map[string]string{
				"ITEM": param,
			}
			params := subDAG.Params
			evaluatedStepParams, err := EvalString(ctx, params, eval.WithVariables(variables))
			if err != nil {
				return nil, fmt.Errorf("failed to eval step params: %w", err)
			}
			finalParams = evaluatedStepParams
		}

		dagRunID := GenerateSubDAGRunID(ctx, finalParams, repeated)
		// Use dagRunID as key to deduplicate - same params will generate same ID
		subRunMap[dagRunID] = SubDAGRun{
			DAGRunID: dagRunID,
			Params:   finalParams,
		}
	}

	// Convert map back to slice
	var subRuns []SubDAGRun
	for _, run := range subRunMap {
		subRuns = append(subRuns, run)
	}

	return subRuns, nil
}

// ItemToParam converts a parallel item to a parameter string
func (n *Node) ItemToParam(item any) (string, error) {
	switch v := item.(type) {
	case string:
		return v, nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	case nil:
		return "null", nil
	case json.RawMessage:
		// Already JSON, return as string
		return string(v), nil
	default:
		// For complex types, marshal to JSON
		data, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal item to JSON: %w", err)
		}
		return string(data), nil
	}
}

type RetryPolicy struct {
	Limit     int
	Interval  time.Duration
	ExitCodes []int
}

// ShouldRetry determines if a node should be retried based on the exit code and retry policy
func (r *RetryPolicy) ShouldRetry(exitCode int) bool {
	if len(r.ExitCodes) > 0 {
		// If exit codes are specified, only retry for those codes
		for _, code := range r.ExitCodes {
			if exitCode == code {
				return true
			}
		}
		return false
	}
	// If no exit codes specified, retry for any non-zero exit code
	return exitCode != 0
}

func (n *Node) setupRetryPolicy(ctx context.Context) error {
	var limit int
	var interval time.Duration
	var exitCodes []int

	step := n.Step()
	if step.RetryPolicy.Limit > 0 {
		limit = step.RetryPolicy.Limit
	}

	if step.RetryPolicy.Interval > 0 {
		interval = step.RetryPolicy.Interval
	}

	if len(step.RetryPolicy.ExitCodes) > 0 {
		exitCodes = step.RetryPolicy.ExitCodes
	}

	// Evaluate the configuration if it's configured as a string
	// e.g. environment variable or command substitution
	if step.RetryPolicy.LimitStr != "" {
		v, err := eval.IntString(ctx, step.RetryPolicy.LimitStr, eval.WithOSExpansion())
		if err != nil {
			return fmt.Errorf("failed to substitute retry limit %q: %w", step.RetryPolicy.LimitStr, err)
		}

		limit = v
	}

	if step.RetryPolicy.IntervalSecStr != "" {
		v, err := eval.IntString(ctx, step.RetryPolicy.IntervalSecStr, eval.WithOSExpansion())
		if err != nil {
			return fmt.Errorf("failed to substitute retry interval %q: %w", step.RetryPolicy.IntervalSecStr, err)
		}

		interval = time.Duration(v) * time.Second
	}

	n.retryPolicy = RetryPolicy{
		Limit:     limit,
		Interval:  interval,
		ExitCodes: exitCodes,
	}

	return nil
}

func (node *Node) evalPreconditions(ctx context.Context) error {
	if len(node.Step().Preconditions) == 0 {
		return nil
	}
	logger.Infof(ctx, "Checking preconditions for \"%s\"", node.Name())
	env := GetEnv(ctx)
	shell := env.Shell(ctx)
	if err := EvalConditions(ctx, shell, node.Step().Preconditions); err != nil {
		logger.Infof(ctx, "Preconditions failed for \"%s\"", node.Name())
		return err
	}
	return nil
}

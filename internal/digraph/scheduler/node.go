package scheduler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"syscall"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// outputCapture handles concurrent reading from a pipe to avoid deadlocks
// when output exceeds the pipe buffer size (typically 64KB)
type outputCapture struct {
	mu            sync.Mutex
	buffer        bytes.Buffer
	done          chan struct{}
	err           error
	maxSize       int64
	bytesRead     int64
	exceededLimit bool
}

// newOutputCapture creates a new output capture handler
func newOutputCapture(maxSize int64) *outputCapture {
	return &outputCapture{
		done:    make(chan struct{}),
		maxSize: maxSize,
	}
}

// start begins reading from the reader concurrently
func (oc *outputCapture) start(ctx context.Context, reader io.Reader) {
	go func() {
		defer close(oc.done)

		// Read in chunks to detect when we exceed the limit
		buf := make([]byte, 8192) // 8KB chunks
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				oc.mu.Lock()
				if oc.bytesRead+int64(n) > oc.maxSize {
					// Only write up to the limit
					remaining := oc.maxSize - oc.bytesRead
					if remaining > 0 {
						oc.buffer.Write(buf[:remaining])
						oc.bytesRead += remaining
					}
					oc.exceededLimit = true
					oc.mu.Unlock()

					// Continue reading to prevent pipe deadlock but discard the data
					_, _ = io.Copy(io.Discard, reader)
					break
				}
				oc.buffer.Write(buf[:n])
				oc.bytesRead += int64(n)
				oc.mu.Unlock()
			}

			if err != nil {
				if err != io.EOF {
					oc.mu.Lock()
					oc.err = fmt.Errorf("failed to read output: %w", err)
					oc.mu.Unlock()
					logger.Error(ctx, "Failed to capture output", "err", err)
				}
				break
			}
		}
	}()
}

// wait waits for the reading to complete and returns the captured output
func (oc *outputCapture) wait() (string, error) {
	<-oc.done

	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.err != nil {
		return "", oc.err
	}

	if oc.exceededLimit {
		return oc.buffer.String(), fmt.Errorf("output exceeded maximum size limit of %d bytes", oc.maxSize)
	}

	return oc.buffer.String(), nil
}

// Node is a node in a DAG. It executes a command.
type Node struct {
	Data
	outputs OutputCoordinator

	id           int
	mu           sync.RWMutex
	cmd          executor.Executor
	cancelFunc   func()
	done         atomic.Bool
	retryPolicy  RetryPolicy
	cmdEvaluated atomic.Bool
}

func NewNode(step digraph.Step, state NodeState) *Node {
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
	case status.NodeSuccess:
		return true

	case status.NodeError:
		if continueOn.Failure {
			return true
		}

	case status.NodeCancel:
		return false

	case status.NodeSkipped:
		if continueOn.Skipped {
			return true
		}
		return false

	case status.NodePartialSuccess:
		// Partial success is treated like success for continue on
		return true

	case status.NodeNone:
		fallthrough

	case status.NodeRunning:
		// Unexpected state
		logger.Error(ctx, "Unexpected node status", "status", s.String())
		return false

	}

	cacheKey := digraph.SystemVariablePrefix + "CONTINUE_ON." + n.Name()
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
			logger.Error(ctx, "Failed to check log for pattern", "err", err)
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
	cmd, err := n.setupExecutor(ctx)
	if err != nil {
		n.SetError(fmt.Errorf("failed to setup executor: %w", err))
		return err
	}

	var exitCode int
	if err := cmd.Run(ctx); err != nil {
		n.SetError(err)

		// Set the exit code if the command implements ExitCoder
		var exitErr *exec.ExitError
		if cmd, ok := cmd.(executor.ExitCoder); ok {
			exitCode = cmd.ExitCode()
		} else if n.Error() != nil && errors.As(n.Error(), &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if code, found := parseExitCodeFromError(n.Error().Error()); found {
			exitCode = code
		} else {
			exitCode = 1
		}
	}

	n.SetExitCode(exitCode)

	// Flush all output writers to ensure data is written before capturing output
	// This is especially important for buffered writers
	_ = n.outputs.flushWriters()

	n.mu.Lock()
	defer n.mu.Unlock()

	// Capture output if configured
	if output := n.Step().Output; output != "" {
		value, err := n.outputs.capturedOutput(ctx)
		if err != nil {
			return fmt.Errorf("failed to capture output: %w", err)
		}
		n.setVariable(output, value)
	}

	if status, ok := cmd.(executor.NodeStatusDeterminer); ok {
		// Determine the node status based on the executor's implementation
		nodeStatus, err := status.DetermineNodeStatus(ctx)
		// Only set the status if it is a success
		// Handle the error case at the scheduler level for simplicity
		if err == nil && nodeStatus.IsSuccess() {
			n.SetStatus(nodeStatus)
		}
	}

	return n.Error()
}

func (n *Node) clearVariable(key string) {
	_ = os.Unsetenv(key)
	n.ClearVariable(key)
}

func (n *Node) setupExecutor(ctx context.Context) (executor.Executor, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	ctx, fn := context.WithCancel(ctx)

	n.cancelFunc = fn

	// Clear the cache
	n.clearVariable(digraph.SystemVariablePrefix + "CONTINUE_ON." + n.Name())

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

	// Evaluate the child DAG if set
	if child := n.Step().ChildDAG; child != nil {
		dagName, err := EvalString(ctx, child.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to eval child DAG name: %w", err)
		}
		copy := *child
		copy.Name = dagName
		n.SetChildDAG(copy)
	}

	// Evaluate script if set
	if script := n.Step().Script; script != "" {
		var opts []cmdutil.EvalOption
		if n.Step().ExecutorConfig.IsCommand() {
			opts = append(opts, cmdutil.OnlyReplaceVars())
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

	// Handle child DAG execution
	if childDAG := n.Step().ChildDAG; childDAG != nil {
		childRuns, err := n.BuildChildDAGRuns(ctx, childDAG)
		if err != nil {
			return nil, err
		}
		n.SetChildRuns(childRuns)

		// Setup the executor with child DAG run information
		if n.Step().Parallel == nil {
			// Single child DAG execution
			exec, ok := cmd.(executor.DAGExecutor)
			if !ok {
				return nil, fmt.Errorf("executor %T does not support child DAG execution", cmd)
			}
			exec.SetParams(executor.RunParams{
				RunID:  childRuns[0].DAGRunID,
				Params: childRuns[0].Params,
			})
		} else {
			// Parallel child DAG execution
			exec, ok := cmd.(executor.ParallelExecutor)
			if !ok {
				return nil, fmt.Errorf("executor %T does not support parallel execution", cmd)
			}
			// Convert ChildDAGRun to executor.RunParams
			var runParamsList []executor.RunParams
			for _, childRun := range childRuns {
				runParamsList = append(runParamsList, executor.RunParams{
					RunID:  childRun.DAGRunID,
					Params: childRun.Params,
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

	var evalOptions []cmdutil.EvalOption

	shellCommand := cmdutil.GetShellCommand(n.Step().Shell)
	if n.Step().ExecutorConfig.IsCommand() && shellCommand != "" {
		// Command executor run commands on shell, so we don't need to expand env vars
		evalOptions = append(evalOptions, cmdutil.WithoutExpandEnv())
	}

	step := n.Step()
	switch {
	case step.CmdArgsSys != "":
		// In case of the command and args are defined as a list. In this case,
		// CmdArgsSys is a string with the command and args separated by special markers.
		cmd, args := cmdutil.SplitCommandArgs(step.CmdArgsSys)
		for i, arg := range args {
			value, err := EvalString(ctx, arg, evalOptions...)
			if err != nil {
				return fmt.Errorf("failed to eval command with args: %w", err)
			}
			args[i] = value
		}
		step.Command = cmd
		step.Args = args

		if step.ExecutorConfig.IsCommand() {
			step.ShellCmdArgs = cmdutil.BuildCommandEscapedString(cmd, args)
		}

	case step.CmdWithArgs != "":
		// In case of the command and args are defined as a string.
		cmdWithArgs, err := EvalString(ctx, step.CmdWithArgs, evalOptions...)
		if err != nil {
			return err
		}

		// Use user defined command as the shell command args that should be already a valid command.
		if step.ExecutorConfig.IsCommand() {
			step.ShellCmdArgs = cmdWithArgs
		}

		// Split the command and args in case shell is not available in the system.
		// In this case, the command and args need to be split to run the command directly.
		cmd, args, err := cmdutil.SplitCommand(cmdWithArgs)
		if err != nil {
			return fmt.Errorf("failed to split command with args: %w", err)
		}

		step.Command = cmd
		step.Args = args

	case step.Command == "":
		// If the command is empty, use the default shell as the command
		step.Command = shellCommand

	case step.Command != "" && len(step.Args) == 0:
		// Shouldn't reach here except for testing.

		cmd, args, err := cmdutil.SplitCommand(step.Command)
		if err != nil {
			return fmt.Errorf("failed to split command: %w", err)
		}
		for i, arg := range args {
			value, err := EvalString(ctx, arg, evalOptions...)
			if err != nil {
				return fmt.Errorf("failed to eval command args: %w", err)
			}
			args[i] = value
		}

		step.CmdWithArgs = step.Command
		step.Command = cmd
		step.Args = args

	default:
		// Shouldn't reach here except for testing.

		if step.Command != "" {
			value, err := EvalString(ctx, step.Command, evalOptions...)
			if err != nil {
				return fmt.Errorf("failed to eval command: %w", err)
			}
			step.Command = value
		}

		for i, arg := range step.Args {
			value, err := EvalString(ctx, arg, evalOptions...)
			if err != nil {
				return fmt.Errorf("failed to eval command args: %w", err)
			}
			step.Args[i] = value
		}
	}

	n.SetStep(step)
	n.cmdEvaluated.Store(true)
	return nil
}

func (n *Node) Signal(ctx context.Context, sig os.Signal, allowOverride bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	s := n.Status()
	if s == status.NodeRunning && n.cmd != nil {
		sigsig := sig
		if allowOverride && n.SignalOnStop() != "" {
			sigsig = syscall.Signal(digraph.GetSignalNum(n.SignalOnStop()))
		}
		logger.Info(ctx, "Sending signal", "signal", sigsig, "step", n.Name())
		if err := n.cmd.Kill(sigsig); err != nil {
			logger.Error(ctx, "Failed to send signal", "err", err, "step", n.Name())
		}
	}
	if s == status.NodeRunning {
		n.SetStatus(status.NodeCancel)
	}
}

func (n *Node) Cancel(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	s := n.Status()
	if s == status.NodeRunning {
		n.SetStatus(status.NodeCancel)
	}
	if n.cancelFunc != nil {
		logger.Info(ctx, "Canceling node", "step", n.Name())
		n.cancelFunc()
	}
}

func (n *Node) SetupContextBeforeExec(ctx context.Context) context.Context {
	n.mu.RLock()
	defer n.mu.RUnlock()
	env := executor.GetEnv(ctx)
	env = env.WithEnv(
		digraph.EnvKeyDAGRunStepStdoutFile, n.GetStdout(),
		digraph.EnvKeyDAGRunStepStderrFile, n.GetStderr(),
	)
	return executor.WithEnv(ctx, env)
}

func (n *Node) Setup(ctx context.Context, logDir string, dagRunID string) error {
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
	if err := n.Data.Setup(ctx, logFile, startedAt); err != nil {
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

func (n *Node) Teardown(ctx context.Context) error {
	if n.done.Load() {
		return nil
	}
	n.done.Store(true)

	var lastErr error
	if err := n.outputs.closeResources(ctx); err != nil {
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
	if env := digraph.GetEnv(ctx); env.DAG != nil && env.DAG.MaxOutputSize > 0 {
		maxOutputSize = env.DAG.MaxOutputSize
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

// BuildChildDAGRuns constructs the child DAG runs based on parallel configuration
func (n *Node) BuildChildDAGRuns(ctx context.Context, childDAG *digraph.ChildDAG) ([]ChildDAGRun, error) {
	parallel := n.Step().Parallel

	// Single child DAG execution (non-parallel)
	if parallel == nil {
		params, err := EvalString(ctx, childDAG.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to eval child dag params: %w", err)
		}
		repeated := n.IsRepeated()
		if repeated && len(n.State().Children) > 0 {
			n.AddChildRunsRepeated(n.State().Children[0])
		}
		dagRunID := GenerateChildDAGRunID(ctx, params, repeated)
		return []ChildDAGRun{{
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
				m := make(digraph.DeterministicMap)
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

	// Build child runs with deduplication
	childRunMap := make(map[string]ChildDAGRun)
	repeated := n.IsRepeated()

	if repeated {
		n.AddChildRunsRepeated(n.State().Children...)
	}

	for i, item := range items {
		param, err := n.ItemToParam(item)
		if err != nil {
			return nil, fmt.Errorf("failed to process item %d: %w", i, err)
		}

		// Merge the item param with the step's params if they exist
		finalParams := param
		if childDAG.Params != "" {
			// Create variables map with ITEM set to the current item value
			variables := map[string]string{
				"ITEM": param,
			}
			params := childDAG.Params
			evaluatedStepParams, err := EvalString(ctx, params, cmdutil.WithVariables(variables))
			if err != nil {
				return nil, fmt.Errorf("failed to eval step params: %w", err)
			}
			finalParams = evaluatedStepParams
		}

		dagRunID := GenerateChildDAGRunID(ctx, finalParams, repeated)
		// Use dagRunID as key to deduplicate - same params will generate same ID
		childRunMap[dagRunID] = ChildDAGRun{
			DAGRunID: dagRunID,
			Params:   finalParams,
		}
	}

	// Convert map back to slice
	var childRuns []ChildDAGRun
	for _, run := range childRunMap {
		childRuns = append(childRuns, run)
	}

	return childRuns, nil
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
		v, err := cmdutil.EvalIntString(ctx, step.RetryPolicy.LimitStr)
		if err != nil {
			return fmt.Errorf("failed to substitute retry limit %q: %w", step.RetryPolicy.LimitStr, err)
		}

		limit = v
	}

	if step.RetryPolicy.IntervalSecStr != "" {
		v, err := cmdutil.EvalIntString(ctx, step.RetryPolicy.IntervalSecStr)
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

type OutputCoordinator struct {
	mu sync.Mutex

	stdoutFileName string
	stdoutFile     *os.File
	stdoutWriter   *bufio.Writer

	stderrFileName string
	stderrFile     *os.File
	stderrWriter   *bufio.Writer

	stdoutRedirectFile   *os.File
	stdoutRedirectWriter *bufio.Writer
	StderrRedirectFile   *os.File
	stderrRedirectWriter *bufio.Writer

	// Output capture with size limits to prevent OOM
	outputWriter   *os.File
	outputReader   *os.File
	outputData     string
	outputCaptured bool
	maxOutputSize  int64          // Max output size in bytes
	outputCapture  *outputCapture // Concurrent output capture handler
}

func (oc *OutputCoordinator) StdoutFile() string {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	return oc.stdoutFileName
}

func (oc *OutputCoordinator) lock() {
	oc.mu.Lock()
}

func (oc *OutputCoordinator) unlock() {
	oc.mu.Unlock()
}

func (oc *OutputCoordinator) setup(ctx context.Context, data NodeData) error {
	if err := oc.setupWriters(ctx, data); err != nil {
		return err
	}
	if err := oc.setupStdoutRedirect(ctx, data); err != nil {
		return err
	}
	return oc.setupStderrRedirect(ctx, data)
}

func (oc *OutputCoordinator) setupExecutorIO(ctx context.Context, cmd executor.Executor, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var stdout io.Writer = os.Stdout
	if oc.stdoutWriter != nil {
		stdout = oc.stdoutWriter
	}

	// Output to both log and stdout
	if oc.stdoutRedirectWriter != nil {
		stdout = newFlushableMultiWriter(oc.stdoutWriter, oc.stdoutRedirectWriter)
	}

	// Setup output capture only if not already set up
	if data.Step.Output != "" && oc.outputReader == nil {
		var err error
		if oc.outputReader, oc.outputWriter, err = os.Pipe(); err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		logger.Debug(ctx, "Created new output pipes", "step", data.Step.Name, "outputVar", data.Step.Output)
		// Reset the captured flag to allow new output capture for retry
		oc.outputCaptured = false

		// Get max output size from DAG configuration, default to 1MB
		oc.maxOutputSize = 1024 * 1024 // 1MB default
		if env := digraph.GetEnv(ctx); env.DAG != nil && env.DAG.MaxOutputSize > 0 {
			oc.maxOutputSize = int64(env.DAG.MaxOutputSize)
		}

		// Reset the output data to empty
		oc.outputData = ""

		// Start concurrent reading to prevent deadlock
		oc.outputCapture = newOutputCapture(oc.maxOutputSize)
		oc.outputCapture.start(ctx, oc.outputReader)
	}

	if oc.outputWriter != nil {
		stdout = newFlushableMultiWriter(stdout, oc.outputWriter)
	}

	cmd.SetStdout(stdout)

	// If stdoutRedirectWriter is set, we write to it as well
	var stderr io.Writer = os.Stderr
	if oc.stderrWriter != nil {
		stderr = oc.stderrWriter
	}
	if oc.stderrRedirectWriter != nil {
		stderr = newFlushableMultiWriter(oc.stderrWriter, oc.stderrRedirectWriter)
	}
	cmd.SetStderr(stderr)

	return nil
}

func (oc *OutputCoordinator) flushWriters() error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var lastErr error
	for _, w := range []*bufio.Writer{oc.stdoutWriter, oc.stderrWriter, oc.stdoutRedirectWriter, oc.stderrRedirectWriter} {
		if w != nil {
			if err := w.Flush(); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

func (oc *OutputCoordinator) closeResources(_ context.Context) error {
	// First flush all writers
	_ = oc.flushWriters()

	oc.mu.Lock()
	defer oc.mu.Unlock()

	var lastErr error

	// Close the output writer first to signal EOF to any readers
	if oc.outputWriter != nil {
		_ = oc.outputWriter.Close()
		oc.outputWriter = nil
	}

	// Wait for concurrent capture to finish if it's running
	if oc.outputCapture != nil && !oc.outputCaptured {
		if _, err := oc.outputCapture.wait(); err != nil {
			lastErr = err
		}
	}

	for _, f := range []*os.File{oc.stdoutFile, oc.stderrFile, oc.stdoutRedirectFile, oc.StderrRedirectFile, oc.outputReader} {
		if f != nil {
			if err := f.Sync(); err != nil {
				lastErr = err
			}
			_ = f.Close()
		}
	}
	return lastErr
}

func (oc *OutputCoordinator) setupStdoutRedirect(ctx context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if data.Step.Stdout == "" {
		return nil
	}

	file, err := oc.setupFile(ctx, data.Step.Stdout, data)
	if err != nil {
		return fmt.Errorf("failed to setup stdout file: %w", err)
	}

	oc.stdoutRedirectFile = file
	oc.stdoutRedirectWriter = bufio.NewWriter(oc.stdoutRedirectFile)

	return nil
}

func (oc *OutputCoordinator) setupStderrRedirect(ctx context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if data.Step.Stderr == "" {
		return nil
	}

	file, err := oc.setupFile(ctx, data.Step.Stderr, data)
	if err != nil {
		return fmt.Errorf("failed to setup stderr file: %w", err)
	}

	oc.StderrRedirectFile = file
	oc.stderrRedirectWriter = bufio.NewWriter(oc.StderrRedirectFile)

	return nil
}

func (oc *OutputCoordinator) setupWriters(_ context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	// stdout
	var err error
	oc.stdoutFile, err = fileutil.OpenOrCreateFile(data.State.Stdout)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	oc.stdoutWriter = bufio.NewWriter(oc.stdoutFile)
	oc.stdoutFileName = data.State.Stdout

	// stderr
	oc.stderrFile, err = fileutil.OpenOrCreateFile(data.State.Stderr)
	if err != nil {
		return fmt.Errorf("failed to open stderr file: %w", err)
	}
	oc.stderrWriter = bufio.NewWriter(oc.stderrFile)
	oc.stderrFileName = data.State.Stderr

	return nil
}

func (oc *OutputCoordinator) setupFile(ctx context.Context, filePath string, _ NodeData) (*os.File, error) {
	absFilePath := filePath
	if !filepath.IsAbs(absFilePath) {
		dir := executor.GetEnv(ctx).WorkingDir
		absFilePath = filepath.Join(dir, absFilePath)
		absFilePath = filepath.Clean(absFilePath)
	}

	file, err := fileutil.OpenOrCreateFile(absFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", absFilePath, err)
	}

	return file, nil
}

func (oc *OutputCoordinator) capturedOutput(ctx context.Context) (string, error) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	// Return cached result if already captured
	if oc.outputCaptured {
		logger.Debug(ctx, "capturedOutput: returning cached", "output", oc.outputData, "length", len(oc.outputData))
		return oc.outputData, nil
	}

	// If using concurrent capture, wait for it to complete
	if oc.outputCapture != nil {
		// Close the writer to signal EOF to the reader
		if oc.outputWriter != nil {
			logger.Debug(ctx, "capturedOutput: closing output writer")
			if err := oc.outputWriter.Close(); err != nil {
				logger.Error(ctx, "failed to close pipe writer", "err", err)
			}
			oc.outputWriter = nil // Mark as closed
		}

		// Wait for the concurrent reader to finish
		output, err := oc.outputCapture.wait()
		if err != nil {
			return "", err
		}

		// Accumulate output with previous attempts (for retries)
		if oc.outputData != "" && output != "" {
			oc.outputData += "\n" + strings.TrimSpace(output)
		} else if output != "" {
			oc.outputData = strings.TrimSpace(output)
		}

		logger.Debug(ctx, "capturedOutput: captured", "output", oc.outputData, "length", len(oc.outputData))

		// Mark as captured for caching
		oc.outputCaptured = true

		// Close the reader
		if oc.outputReader != nil {
			if err := oc.outputReader.Close(); err != nil {
				logger.Error(ctx, "failed to close pipe reader", "err", err)
			}
			oc.outputReader = nil
		}

		return oc.outputData, nil
	}

	// Fallback to old behavior if concurrent capture not used (shouldn't happen)
	if oc.outputReader == nil {
		logger.Debug(ctx, "capturedOutput: no output reader")
		return "", nil
	}

	// Close the writer only if it hasn't been closed already
	if oc.outputWriter != nil {
		logger.Debug(ctx, "capturedOutput: closing output writer")
		if err := oc.outputWriter.Close(); err != nil {
			logger.Error(ctx, "failed to close pipe writer", "err", err)
		}
		oc.outputWriter = nil // Mark as closed
	}

	// Use limited reader to prevent OOM
	limitedReader := io.LimitReader(oc.outputReader, oc.maxOutputSize)
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, limitedReader); err != nil {
		return "", fmt.Errorf("io: failed to read output: %w", err)
	}

	output := strings.TrimSpace(buf.String())

	// Check if output was truncated
	if buf.Len() == int(oc.maxOutputSize) {
		logger.Warn(ctx, "Output truncated due to size limit", "maxSize", oc.maxOutputSize)
		output += "\n[OUTPUT TRUNCATED]"
	}

	// Accumulate output with previous attempts (for retries)
	if oc.outputData != "" && output != "" {
		oc.outputData += "\n" + output
	} else if output != "" {
		oc.outputData = output
	}

	logger.Debug(ctx, "capturedOutput: captured", "output", oc.outputData, "length", len(oc.outputData))

	// Close the reader after reading
	if err := oc.outputReader.Close(); err != nil {
		logger.Error(ctx, "failed to close pipe reader", "err", err)
	}
	oc.outputReader = nil // Mark as closed

	// Mark as captured for caching
	oc.outputCaptured = true

	return oc.outputData, nil
}

func (node *Node) evalPreconditions(ctx context.Context) error {
	if len(node.Step().Preconditions) == 0 {
		return nil
	}
	logger.Infof(ctx, "Checking preconditions for \"%s\"", node.Name())
	shell := cmdutil.GetShellCommand(node.Step().Shell)
	if err := EvalConditions(ctx, shell, node.Step().Preconditions); err != nil {
		logger.Infof(ctx, "Preconditions failed for \"%s\"", node.Name())
		return err
	}
	return nil
}

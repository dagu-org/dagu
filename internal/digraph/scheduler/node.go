package scheduler

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/stringutil"
)

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

func (n *Node) shouldMarkSuccess(ctx context.Context) bool {
	if !n.shouldContinue(ctx) {
		return false
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.ContinueOn().MarkSuccess
}

func (n *Node) shouldContinue(ctx context.Context) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	continueOn := n.ContinueOn()

	status := n.Status()
	switch status {
	case NodeStatusSuccess:
		return true

	case NodeStatusError:
		if continueOn.Failure {
			return true
		}
	case NodeStatusCancel:
		return false

	case NodeStatusSkipped:
		if continueOn.Skipped {
			return true
		}

	case NodeStatusNone:
		fallthrough

	case NodeStatusRunning:
		// Unexpected state
		logger.Error(ctx, "Unexpected node status", "status", status.String())
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
		return err
	}

	var exitCode int
	if err := cmd.Run(ctx); err != nil {
		n.SetError(err)

		// Set the exit code if the command implements ExitCoder
		if cmd, ok := cmd.(executor.ExitCoder); ok {
			exitCode = cmd.ExitCode()
		} else {
			exitCode = 1
		}
	}

	n.SetExitCode(exitCode)

	n.mu.Lock()
	defer n.mu.Unlock()

	if output := n.Step().Output; output != "" {
		value, err := n.outputs.capturedOutput(ctx)
		if err != nil {
			return fmt.Errorf("failed to capture output: %w", err)
		}
		n.setVariable(output, value)
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
		childDAG, err := EvalObject(ctx, *child)
		if err != nil {
			return nil, fmt.Errorf("failed to eval child DAG: %w", err)
		}
		n.SetChildDAG(childDAG)
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

	// If the command is a child DAG, set the DAG-run ID
	if childDAG, ok := cmd.(executor.ChildDAG); ok {
		dagRunID, err := n.GenChildDAGRunID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate child DAG run ID: %w", err)
		}
		childDAG.SetDAGRunID(dagRunID)
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

	status := n.Status()
	if status == NodeStatusRunning && n.cmd != nil {
		sigsig := sig
		if allowOverride && n.SignalOnStop() != "" {
			sigsig = unix.SignalNum(n.SignalOnStop())
		}
		logger.Info(ctx, "Sending signal", "signal", sigsig, "step", n.Name())
		if err := n.cmd.Kill(sigsig); err != nil {
			logger.Error(ctx, "Failed to send signal", "err", err, "step", n.Name())
		}
	}
	if status == NodeStatusRunning {
		n.SetStatus(NodeStatusCancel)
	}
}

func (n *Node) Cancel(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.Status()
	if status == NodeStatusRunning {
		n.SetStatus(NodeStatusCancel)
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
	timestamp := startedAt.Format("20060102.15:04:05.000")
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

	// Create a buffered reader with optimal buffer size
	reader := bufio.NewReaderSize(file, 64*1024)

	// Use scanner for more efficient line reading
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // Set max line size to 1MB

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

	outputWriter *os.File
	outputReader *os.File
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

func (oc *OutputCoordinator) setupExecutorIO(_ context.Context, cmd executor.Executor, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var stdout io.Writer = os.Stdout
	if oc.stdoutWriter != nil {
		stdout = oc.stdoutWriter
	}

	// Output to both log and stdout
	if oc.stdoutRedirectWriter != nil {
		stdout = io.MultiWriter(oc.stdoutWriter, oc.stdoutRedirectWriter)
	}

	// Setup output capture
	if data.Step.Output != "" {
		var err error
		if oc.outputReader, oc.outputWriter, err = os.Pipe(); err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		stdout = io.MultiWriter(stdout, oc.outputWriter)
	}

	cmd.SetStdout(stdout)

	// If stdoutRedirectWriter is set, we write to it as well
	var stderr io.Writer = os.Stderr
	if oc.stderrWriter != nil {
		stderr = oc.stderrWriter
	}
	if oc.stderrRedirectWriter != nil {
		stderr = io.MultiWriter(oc.stderrWriter, oc.stderrRedirectWriter)
	}
	cmd.SetStderr(stderr)

	return nil
}

func (oc *OutputCoordinator) closeResources(_ context.Context) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var lastErr error
	for _, w := range []*bufio.Writer{oc.stdoutWriter, oc.stdoutRedirectWriter, oc.stderrRedirectWriter} {
		if w != nil {
			if err := w.Flush(); err != nil {
				lastErr = err
			}
		}
	}
	for _, f := range []*os.File{oc.stdoutFile, oc.stdoutRedirectFile, oc.StderrRedirectFile} {
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

func (oc *OutputCoordinator) setupFile(_ context.Context, filePath string, data NodeData) (*os.File, error) {
	absFilePath := filePath
	if !filepath.IsAbs(absFilePath) {
		absFilePath = filepath.Join(data.Step.Dir, absFilePath)
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

	if oc.outputReader == nil {
		return "", nil
	}

	if err := oc.outputWriter.Close(); err != nil {
		logger.Error(ctx, "failed to close pipe writer", "err", err)
	}

	var buf bytes.Buffer
	// TODO: Handle case where output is too large
	if _, err := io.Copy(&buf, oc.outputReader); err != nil {
		return "", fmt.Errorf("io: failed to read output: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

package scheduler

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	data NodeData

	id           int
	mu           sync.RWMutex
	logLock      sync.Mutex
	cmd          executor.Executor
	cancelFunc   func()
	logFile      *os.File
	logWriter    *bufio.Writer
	stdoutFile   *os.File
	stdoutWriter *bufio.Writer
	stderrFile   *os.File
	stderrWriter *bufio.Writer
	outputWriter *os.File
	outputReader *os.File
	scriptFile   *os.File
	done         bool
	retryPolicy  retryPolicy
	cmdEvaluated bool
}

type NodeData struct {
	Step  digraph.Step
	State NodeState
}

// NodeState contains the state of a node.
type NodeState struct {
	Status     NodeStatus
	Log        string
	StartedAt  time.Time
	FinishedAt time.Time
	RetryCount int
	RetriedAt  time.Time
	DoneCount  int
	Error      error
	ExitCode   int
}

// NodeStatus represents the status of a node.
type NodeStatus int

const (
	NodeStatusNone NodeStatus = iota
	NodeStatusRunning
	NodeStatusError
	NodeStatusCancel
	NodeStatusSuccess
	NodeStatusSkipped
)

func (s NodeStatus) String() string {
	switch s {
	case NodeStatusRunning:
		return "running"
	case NodeStatusError:
		return "failed"
	case NodeStatusCancel:
		return "canceled"
	case NodeStatusSuccess:
		return "finished"
	case NodeStatusSkipped:
		return "skipped"
	case NodeStatusNone:
		fallthrough
	default:
		return "not started"
	}
}

func NodeWithData(data NodeData) *Node {
	return &Node{
		data: data,
	}
}

func NewNode(step digraph.Step, state NodeState) *Node {
	return &Node{
		data: NodeData{Step: step, State: state},
	}
}

func (n *Node) Data() NodeData {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data
}

func (n *Node) ScriptFilename() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.scriptFile != nil {
		return n.scriptFile.Name()
	}
	return ""
}

func (n *Node) CloseLog() error {
	n.logLock.Lock()
	defer n.logLock.Unlock()
	if n.logFile != nil {
		return n.logFile.Close()
	}
	return nil
}

func (n *Node) LogFilename() string {
	n.logLock.Lock()
	defer n.logLock.Unlock()
	if n.logFile != nil {
		return n.logFile.Name()
	}
	return ""
}

func (n *Node) shouldMarkSuccess(ctx context.Context) bool {
	if !n.shouldContinue(ctx) {
		return false
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data.Step.ContinueOn.MarkSuccess
}

func (n *Node) shouldContinue(ctx context.Context) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	continueOn := n.data.Step.ContinueOn

	switch n.data.State.Status {
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
		logger.Error(ctx, "unexpected node status", "status", n.data.State.Status)
		return false
	}

	cacheKey := digraph.SystemVariablePrefix + "CONTINUE_ON." + n.data.Step.Name

	if v, ok := n.getBoolVariable(cacheKey); ok {
		return v
	}

	// If the exit code is in the list, continue
	if len(continueOn.ExitCode) > 0 {
		var found bool
		exitCode := n.data.State.ExitCode
		for _, code := range continueOn.ExitCode {
			if code == exitCode {
				found = true
				break
			}
		}
		if found {
			// cache the result
			n.setBoolVariable(cacheKey, true)
			return true
		}
	}

	if len(continueOn.Output) > 0 {
		ok, err := n.LogContainsPattern(ctx, continueOn.Output)
		if err != nil {
			logger.Error(ctx, "failed to check log for pattern", "err", err)
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

func (n *Node) setError(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.Error = err
}

func (n *Node) State() NodeState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data.State
}

// Execute runs the command synchronously and returns error if any.
func (n *Node) Execute(ctx context.Context) error {
	cmd, err := n.setupExec(ctx)
	if err != nil {
		return err
	}

	var exitCode int
	if err := cmd.Run(ctx); err != nil {
		n.setError(err)

		// Set the exit code if the command implements ExitCoder
		if cmd, ok := cmd.(executor.ExitCoder); ok {
			exitCode = cmd.ExitCode()
		} else {
			exitCode = 1
		}
	}

	n.SetExitCode(exitCode)

	if n.outputReader != nil && n.data.Step.Output != "" {
		if err := n.outputWriter.Close(); err != nil {
			logger.Error(ctx, "failed to close pipe writer", "err", err)
		}
		var buf bytes.Buffer
		// TODO: handle the case where the error or output is too large
		_, _ = io.Copy(&buf, n.outputReader)
		value := strings.TrimSpace(buf.String())
		n.setVariable(n.data.Step.Output, value)
	}

	return n.data.State.Error
}

func (n *Node) clearVariable(key string) {
	_ = os.Unsetenv(key)

	if n.data.Step.OutputVariables == nil {
		return
	}
	n.data.Step.OutputVariables.Delete(key)
}

func (n *Node) getVariable(key string) (stringutil.KeyValue, bool) {
	if n.data.Step.OutputVariables == nil {
		return "", false
	}
	v, ok := n.data.Step.OutputVariables.Load(key)
	if !ok {
		return "", false
	}
	return stringutil.KeyValue(v.(string)), true
}

func (n *Node) getBoolVariable(key string) (bool, bool) {
	v, ok := n.getVariable(key)
	if !ok {
		return false, false
	}
	return v.Bool(), true
}

func (n *Node) setBoolVariable(key string, value bool) {
	if n.data.Step.OutputVariables == nil {
		n.data.Step.OutputVariables = &digraph.SyncMap{}
	}
	n.data.Step.OutputVariables.Store(key, stringutil.NewKeyValue(key, strconv.FormatBool(value)).String())
}

func (n *Node) setVariable(key, value string) {
	if n.data.Step.OutputVariables == nil {
		n.data.Step.OutputVariables = &digraph.SyncMap{}
	}
	n.data.Step.OutputVariables.Store(key, stringutil.NewKeyValue(key, value).String())
}

func (n *Node) Finish() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.FinishedAt = time.Now()
}

func (n *Node) setupExec(ctx context.Context) (executor.Executor, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	ctx, fn := context.WithCancel(ctx)

	n.cancelFunc = fn

	// Clear the cache
	n.clearVariable(digraph.SystemVariablePrefix + "CONTINUE_ON." + n.data.Step.Name)

	// Reset the state
	n.data.State.Error = nil
	n.data.State.ExitCode = 0

	// Evaluate the command and args if not already evaluated
	if err := n.evaluateCommandArgs(ctx); err != nil {
		return nil, err
	}

	if n.scriptFile != nil {
		var args []string
		args = append(args, n.data.Step.Args...)
		n.data.Step.Args = append(args, n.scriptFile.Name())
	}

	cmd, err := executor.NewExecutor(ctx, n.data.Step)
	if err != nil {
		return nil, err
	}
	n.cmd = cmd

	var stdout io.Writer

	if n.logWriter != nil {
		stdout = n.logWriter
		cmd.SetStderr(stdout)
	}

	if n.stdoutWriter != nil {
		stdout = io.MultiWriter(n.logWriter, n.stdoutWriter)
	}

	if n.data.Step.Output != "" {
		var err error
		if n.outputReader, n.outputWriter, err = os.Pipe(); err != nil {
			return nil, err
		}
		stdout = io.MultiWriter(stdout, n.outputWriter)
	}

	cmd.SetStdout(stdout)
	if n.stderrWriter != nil {
		cmd.SetStderr(n.stderrWriter)
	} else {
		cmd.SetStderr(stdout)
	}

	return cmd, nil
}

func (n *Node) evaluateCommandArgs(ctx context.Context) error {
	if n.cmdEvaluated {
		return nil
	}

	stepContext := digraph.GetStepContext(ctx)
	switch {
	case n.data.Step.CmdArgsSys != "":
		// In case of the command and args are defined as a list. In this case,
		// CmdArgsSys is a string with the command and args separated by special markers.
		cmd, args := cmdutil.SplitCommandArgs(n.data.Step.CmdArgsSys)
		for i, arg := range args {
			value, err := stepContext.EvalString(arg, cmdutil.WithoutExpandEnv())
			if err != nil {
				return fmt.Errorf("failed to eval command with args: %w", err)
			}
			args[i] = value
		}
		n.data.Step.Command = cmd
		n.data.Step.Args = args

		if n.data.Step.ExecutorConfig.IsCommand() {
			n.data.Step.ShellCmdArgs = cmdutil.BuildCommandEscapedString(cmd, args)
		}

	case n.data.Step.CmdWithArgs != "":
		// In case of the command and args are defined as a string.
		stepContext := digraph.GetStepContext(ctx)
		cmdWithArgs, err := stepContext.EvalString(n.data.Step.CmdWithArgs, cmdutil.WithoutExpandEnv())
		if err != nil {
			return err
		}

		// Use user defined command as the shell command args that should be already a valid command.
		if n.data.Step.ExecutorConfig.IsCommand() {
			n.data.Step.ShellCmdArgs = cmdWithArgs
		}

		// Split the command and args in case shell is not available in the system.
		// In this case, the command and args need to be split to run the command directly.
		cmd, args, err := cmdutil.SplitCommand(cmdWithArgs)
		if err != nil {
			return fmt.Errorf("failed to split command with args: %w", err)
		}

		n.data.Step.Command = cmd
		n.data.Step.Args = args

	case n.data.Step.Command == "":
		// If the command is empty, use the default shell as the command
		n.data.Step.Command = cmdutil.GetShellCommand(n.data.Step.Shell)

	case n.data.Step.Command != "" && len(n.data.Step.Args) == 0:
		// Shouldn't reach here except for testing.

		cmd, args, err := cmdutil.SplitCommand(n.data.Step.Command)
		if err != nil {
			return fmt.Errorf("failed to split command: %w", err)
		}
		for i, arg := range args {
			value, err := stepContext.EvalString(arg, cmdutil.WithoutExpandEnv())
			if err != nil {
				return fmt.Errorf("failed to eval command args: %w", err)
			}
			args[i] = value
		}

		n.data.Step.CmdWithArgs = n.data.Step.Command
		n.data.Step.Command = cmd
		n.data.Step.Args = args

	default:
		// Shouldn't reach here except for testing.

		if n.data.Step.Command != "" {
			value, err := stepContext.EvalString(n.data.Step.Command, cmdutil.WithoutExpandEnv())
			if err != nil {
				return fmt.Errorf("failed to eval command: %w", err)
			}
			n.data.Step.Command = value
		}

		for i, arg := range n.data.Step.Args {
			value, err := stepContext.EvalString(arg, cmdutil.WithoutExpandEnv())
			if err != nil {
				return fmt.Errorf("failed to eval command args: %w", err)
			}
			n.data.Step.Args[i] = value
		}
	}

	n.cmdEvaluated = true
	return nil
}

func (n *Node) GetRetryCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data.State.RetryCount
}

func (n *Node) SetRetriedAt(retriedAt time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.RetriedAt = retriedAt
}

func (n *Node) GetDoneCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data.State.DoneCount
}

func (n *Node) GetExitCode() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data.State.ExitCode
}

func (n *Node) SetExitCode(exitCode int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.ExitCode = exitCode
}

func (n *Node) ClearState() {
	n.data.State = NodeState{}
}

func (n *Node) SetStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.Status = status
}

func (n *Node) MarkError(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.Error = err
	n.data.State.Status = NodeStatusError
}

func (n *Node) Signal(ctx context.Context, sig os.Signal, allowOverride bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.data.State.Status
	if status == NodeStatusRunning && n.cmd != nil {
		sigsig := sig
		if allowOverride && n.data.Step.SignalOnStop != "" {
			sigsig = unix.SignalNum(n.data.Step.SignalOnStop)
		}
		logger.Info(ctx, "Sending signal", "signal", sigsig, "step", n.data.Step.Name)
		if err := n.cmd.Kill(sigsig); err != nil {
			logger.Error(ctx, "Failed to send signal", "err", err, "step", n.data.Step.Name)
		}
	}
	if status == NodeStatusRunning {
		n.data.State.Status = NodeStatusCancel
	}
}

func (n *Node) Cancel(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.data.State.Status
	if status == NodeStatusRunning {
		n.data.State.Status = NodeStatusCancel
	}
	if n.cancelFunc != nil {
		logger.Info(ctx, "canceling node", "step", n.data.Step.Name)
		n.cancelFunc()
	}
}

func (n *Node) SetupContextBeforeExec(ctx context.Context) context.Context {
	n.mu.RLock()
	defer n.mu.RUnlock()

	stepContext := digraph.GetStepContext(ctx)
	stepContext = stepContext.WithEnv(digraph.EnvKeyLogPath, n.data.State.Log)
	stepContext = stepContext.WithEnv(digraph.EnvKeyDAGStepLogPath, n.data.State.Log)

	return digraph.WithStepContext(ctx, stepContext)
}

func (n *Node) Setup(ctx context.Context, logDir string, requestID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	stepContext := digraph.GetStepContext(ctx)

	// Set the log file path
	startedAt := time.Now()

	safeName := fileutil.SafeName(n.data.Step.Name)
	timestamp := startedAt.Format("20060102.15:04:05.000")
	postfix := stringutil.TruncString(requestID, 8)
	logFilename := fmt.Sprintf("%s.%s.%s.log", safeName, timestamp, postfix)
	if !fileutil.FileExists(logDir) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory %q: %w", logDir, err)
		}
	}

	filePath := filepath.Join(logDir, logFilename)
	n.data.State.Log = filePath
	n.data.State.StartedAt = startedAt

	stdout, err := stepContext.EvalString(n.data.Step.Stdout)
	if err != nil {
		return fmt.Errorf("failed to evaluate stdout field: %w", err)
	}
	n.data.Step.Stdout = stdout

	stderr, err := stepContext.EvalString(n.data.Step.Stderr)
	if err != nil {
		return fmt.Errorf("failed to evaluate stderr field: %w", err)
	}
	n.data.Step.Stderr = stderr

	dir, err := stepContext.EvalString(n.data.Step.Dir)
	if err != nil {
		return fmt.Errorf("failed to evaluate dir field: %w", err)
	}
	n.data.Step.Dir = dir

	if err := n.setupLog(); err != nil {
		return fmt.Errorf("failed to setup log: %w", err)
	}
	if err := n.setupStdout(); err != nil {
		return fmt.Errorf("failed to setup stdout: %w", err)
	}
	if err := n.setupStderr(); err != nil {
		return fmt.Errorf("failed to setup stderr: %w", err)
	}
	if err := n.setupRetryPolicy(ctx); err != nil {
		return fmt.Errorf("failed to setup retry policy: %w", err)
	}
	if err := n.setupScript(); err != nil {
		return fmt.Errorf("failed to setup script: %w", err)
	}
	return nil
}

func (n *Node) Teardown() error {
	if n.done {
		return nil
	}
	n.logLock.Lock()
	n.done = true
	var lastErr error
	for _, w := range []*bufio.Writer{n.logWriter, n.stdoutWriter} {
		if w != nil {
			if err := w.Flush(); err != nil {
				lastErr = err
			}
		}
	}
	for _, f := range []*os.File{n.logFile, n.stdoutFile} {
		if f != nil {
			if err := f.Sync(); err != nil {
				lastErr = err
			}
			_ = f.Close()
		}
	}
	n.logLock.Unlock()

	if n.scriptFile != nil {
		_ = os.Remove(n.scriptFile.Name())
	}
	if lastErr != nil {
		n.data.State.Error = lastErr
	}

	return lastErr
}

func (n *Node) IncRetryCount() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.RetryCount++
}

func (n *Node) IncDoneCount() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.DoneCount++
}

var (
	ErrWorkingDirNotExist = fmt.Errorf("working directory does not exist")
)

func (n *Node) setupScript() (err error) {
	if n.data.Step.Script != "" {
		if len(n.data.Step.Dir) > 0 && !fileutil.FileExists(n.data.Step.Dir) {
			return ErrWorkingDirNotExist
		}
		n.scriptFile, _ = os.CreateTemp(n.data.Step.Dir, "dagu_script-")
		if _, err = n.scriptFile.WriteString(n.data.Step.Script); err != nil {
			return err
		}
		defer func() {
			_ = n.scriptFile.Close()
		}()
		err = n.scriptFile.Sync()
	}
	return err
}

func (n *Node) setupStdout() error {
	if n.data.Step.Stdout != "" {
		f := n.data.Step.Stdout
		if !filepath.IsAbs(f) {
			f = filepath.Join(n.data.Step.Dir, f)
		}
		var err error
		n.stdoutFile, err = fileutil.OpenOrCreateFile(f)
		if err != nil {
			n.data.State.Error = err
			return err
		}
		n.stdoutWriter = bufio.NewWriter(n.stdoutFile)
	}
	return nil
}

func (n *Node) setupStderr() error {
	if n.data.Step.Stderr != "" {
		f := n.data.Step.Stderr
		if !filepath.IsAbs(f) {
			f = filepath.Join(n.data.Step.Dir, f)
		}
		var err error
		n.stderrFile, err = fileutil.OpenOrCreateFile(f)
		if err != nil {
			n.data.State.Error = err
			return err
		}
		n.stderrWriter = bufio.NewWriter(n.stderrFile)
	}
	return nil
}

func (n *Node) setupLog() error {
	if n.data.State.Log == "" {
		return nil
	}
	n.logLock.Lock()
	defer n.logLock.Unlock()
	var err error
	n.logFile, err = fileutil.OpenOrCreateFile(n.data.State.Log)
	if err != nil {
		n.data.State.Error = err
		return err
	}
	n.logWriter = bufio.NewWriter(n.logFile)
	return nil
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
	logFilename := n.LogFilename()
	if logFilename == "" {
		return false, nil
	}

	// Open the log file
	file, err := os.Open(logFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Create a buffered reader with optimal buffer size
	reader := bufio.NewReaderSize(file, 64*1024)

	// Use scanner for more efficient line reading
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // Set max line size to 1MB

	// Use the logLock to prevent concurrent file operations
	n.logLock.Lock()
	defer n.logLock.Unlock()

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
	if n.data.Step.Preconditions == nil {
		n.data.Step.Preconditions = []digraph.Condition{}
	}
}

type retryPolicy struct {
	Limit    int
	Interval time.Duration
}

func (n *Node) setupRetryPolicy(ctx context.Context) error {
	var retryPolicy retryPolicy

	if n.data.Step.RetryPolicy.Limit > 0 {
		retryPolicy.Limit = n.data.Step.RetryPolicy.Limit
	}
	if n.data.Step.RetryPolicy.Interval > 0 {
		retryPolicy.Interval = n.data.Step.RetryPolicy.Interval
	}
	// Evaluate the configuration if it's configured as a string
	// e.g. environment variable or command substitution
	if n.data.Step.RetryPolicy.LimitStr != "" {
		v, err := cmdutil.EvalIntString(ctx, n.data.Step.RetryPolicy.LimitStr)
		if err != nil {
			return fmt.Errorf("failed to substitute retry limit %q: %w", n.data.Step.RetryPolicy.LimitStr, err)
		}
		retryPolicy.Limit = v
	}
	if n.data.Step.RetryPolicy.IntervalSecStr != "" {
		v, err := cmdutil.EvalIntString(ctx, n.data.Step.RetryPolicy.IntervalSecStr)
		if err != nil {
			return fmt.Errorf("failed to substitute retry interval %q: %w", n.data.Step.RetryPolicy.IntervalSecStr, err)
		}
		retryPolicy.Interval = time.Duration(v) * time.Second
	}
	n.retryPolicy = retryPolicy
	return nil
}

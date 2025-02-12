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
	data    SafeData
	outputs OutputCoordinator

	id           int
	mu           sync.RWMutex
	cmd          executor.Executor
	cancelFunc   func()
	script       *Script
	done         atomic.Bool
	retryPolicy  RetryPolicy
	cmdEvaluated atomic.Bool
}

type SafeData struct {
	mu   sync.RWMutex
	data NodeData
}

func newSafeData(data NodeData) SafeData {
	return SafeData{data: data}
}

func (s *SafeData) ResetError() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.State.Error = nil
	s.data.State.ExitCode = 0
}

func (s *SafeData) Args() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	args := make([]string, len(s.data.Step.Args))
	copy(args, s.data.Step.Args)
	return args
}

func (s *SafeData) SetArgs(args []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Step.Args = args
}

func (s *SafeData) Step() digraph.Step {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.Step
}

func (s *SafeData) SetStep(step digraph.Step) {
	// TODO: refactor to avoid modifying the step
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Step = step
}

func (s *SafeData) Data() NodeData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data
}

func (s *SafeData) Setup(ctx context.Context, logFile string, startedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.State.Log = logFile
	s.data.State.StartedAt = startedAt

	stepContext := digraph.GetStepContext(ctx)

	// Evaluate the stdout and stderr fields
	stdout, err := stepContext.EvalString(s.data.Step.Stdout)
	if err != nil {
		return fmt.Errorf("failed to evaluate stdout field: %w", err)
	}
	s.data.Step.Stdout = stdout

	stderr, err := stepContext.EvalString(s.data.Step.Stderr)
	if err != nil {
		return fmt.Errorf("failed to evaluate stderr field: %w", err)
	}
	s.data.Step.Stderr = stderr

	// Evaluate the dir field
	dir, err := stepContext.EvalString(s.data.Step.Dir)
	if err != nil {
		return fmt.Errorf("failed to evaluate dir field: %w", err)
	}
	s.data.Step.Dir = dir

	return nil
}

func (s *SafeData) State() NodeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.State
}

func (s *SafeData) Status() NodeStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.State.Status
}

func (s *SafeData) SetStatus(status NodeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.State.Status = status
}

func (s *SafeData) ContinueOn() digraph.ContinueOn {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.Step.ContinueOn
}

func (s *SafeData) Log() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.State.Log
}

func (s *SafeData) SignalOnStop() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.Step.SignalOnStop
}

func (s *SafeData) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.Step.Name
}

func (s *SafeData) Error() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data.State.Error
}

func (s *SafeData) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.State.Error = err
}

func (s *SafeData) ClearVariable(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.Step.OutputVariables == nil {
		return
	}

	s.data.Step.OutputVariables.Delete(key)
}

func (s *SafeData) MatchExitCode(exitCodes []int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, code := range exitCodes {
		if code == s.data.State.ExitCode {
			return true
		}
	}
	return false
}

func NewNode(step digraph.Step, state NodeState) *Node {
	return &Node{
		data: newSafeData(NodeData{Step: step, State: state}),
	}
}

func NodeWithData(data NodeData) *Node {
	return &Node{
		data: newSafeData(data),
	}
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

func (n *Node) Data() NodeData {
	return n.data.Data()
}

func (n *Node) LogFile() string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.outputs.LogFile()
}

func (n *Node) SetStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.data.SetStatus(status)
}

func (n *Node) Script() *Script {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.script
}

func (n *Node) shouldMarkSuccess(ctx context.Context) bool {
	if !n.shouldContinue(ctx) {
		return false
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data.ContinueOn().MarkSuccess
}

func (n *Node) shouldContinue(ctx context.Context) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	continueOn := n.data.ContinueOn()

	status := n.data.Status()
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
		logger.Error(ctx, "unexpected node status", "status", status)
		return false

	}

	cacheKey := digraph.SystemVariablePrefix + "CONTINUE_ON." + n.data.Name()
	if v, ok := n.data.getBoolVariable(cacheKey); ok {
		return v
	}

	if n.data.MatchExitCode(continueOn.ExitCode) {
		n.data.setBoolVariable(cacheKey, true)
		return true
	}

	if len(continueOn.Output) > 0 {
		ok, err := n.LogContainsPattern(ctx, continueOn.Output)
		if err != nil {
			logger.Error(ctx, "failed to check log for pattern", "err", err)
			return false
		}
		if ok {
			n.data.setBoolVariable(cacheKey, true)
			return true
		}
	}

	n.data.setBoolVariable(cacheKey, false)
	return false
}

func (n *Node) State() NodeState {
	return n.data.State()
}

func (n *Node) Execute(ctx context.Context) error {
	cmd, err := n.setupExec(ctx)
	if err != nil {
		return err
	}

	var exitCode int
	if err := cmd.Run(ctx); err != nil {
		n.data.SetError(err)

		// Set the exit code if the command implements ExitCoder
		if cmd, ok := cmd.(executor.ExitCoder); ok {
			exitCode = cmd.ExitCode()
		} else {
			exitCode = 1
		}
	}

	n.data.SetExitCode(exitCode)

	n.mu.Lock()
	defer n.mu.Unlock()

	if output := n.data.Step().Output; output != "" {
		value, err := n.outputs.capturedOutput(ctx)
		if err != nil {
			return fmt.Errorf("failed to capture output: %w", err)
		}
		n.data.setVariable(output, value)
	}

	return n.data.Error()
}

func (n *Node) clearVariable(key string) {
	_ = os.Unsetenv(key)
	n.data.ClearVariable(key)
}

func (n *SafeData) getVariable(key string) (stringutil.KeyValue, bool) {
	// n.mu.RLock()
	// defer n.mu.RUnlock()

	if n.data.Step.OutputVariables == nil {
		return "", false
	}

	v, ok := n.data.Step.OutputVariables.Load(key)
	if !ok {
		return "", false
	}

	return stringutil.KeyValue(v.(string)), true
}

func (n *SafeData) getBoolVariable(key string) (bool, bool) {
	v, ok := n.getVariable(key)
	if !ok {
		return false, false
	}

	return v.Bool(), true
}

func (n *SafeData) setBoolVariable(key string, value bool) {
	if n.data.Step.OutputVariables == nil {
		n.data.Step.OutputVariables = &digraph.SyncMap{}
	}
	n.data.Step.OutputVariables.Store(key, stringutil.NewKeyValue(key, strconv.FormatBool(value)).String())
}

func (n *SafeData) setVariable(key, value string) {
	if n.data.Step.OutputVariables == nil {
		n.data.Step.OutputVariables = &digraph.SyncMap{}
	}
	n.data.Step.OutputVariables.Store(key, stringutil.NewKeyValue(key, value).String())
}

func (n *SafeData) Finish() {
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
	n.clearVariable(digraph.SystemVariablePrefix + "CONTINUE_ON." + n.data.Name())

	// Reset the state
	n.data.ResetError()

	// Reset the done flag
	n.done.Store(false)

	// Evaluate the command and args if not already evaluated
	if err := n.evaluateCommandArgs(ctx); err != nil {
		return nil, err
	}

	if n.script != nil {
		args := n.data.Args()
		args = append(args, n.script.Name())
		n.data.SetArgs(args)
	}

	cmd, err := executor.NewExecutor(ctx, n.data.Step())
	if err != nil {
		return nil, err
	}
	n.cmd = cmd

	if err := n.outputs.setupExecutorIO(ctx, cmd, n.data.Data()); err != nil {
		return nil, fmt.Errorf("failed to setup executor IO: %w", err)
	}

	return cmd, nil
}

func (n *Node) evaluateCommandArgs(ctx context.Context) error {
	if n.cmdEvaluated.Load() {
		return nil
	}

	stepContext := digraph.GetStepContext(ctx)
	step := n.data.Step()
	switch {
	case step.CmdArgsSys != "":
		// In case of the command and args are defined as a list. In this case,
		// CmdArgsSys is a string with the command and args separated by special markers.
		cmd, args := cmdutil.SplitCommandArgs(step.CmdArgsSys)
		for i, arg := range args {
			value, err := stepContext.EvalString(arg, cmdutil.WithoutExpandEnv())
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
		stepContext := digraph.GetStepContext(ctx)
		cmdWithArgs, err := stepContext.EvalString(step.CmdWithArgs, cmdutil.WithoutExpandEnv())
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
		step.Command = cmdutil.GetShellCommand(step.Shell)

	case step.Command != "" && len(step.Args) == 0:
		// Shouldn't reach here except for testing.

		cmd, args, err := cmdutil.SplitCommand(step.Command)
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

		step.CmdWithArgs = step.Command
		step.Command = cmd
		step.Args = args

	default:
		// Shouldn't reach here except for testing.

		if step.Command != "" {
			value, err := stepContext.EvalString(step.Command, cmdutil.WithoutExpandEnv())
			if err != nil {
				return fmt.Errorf("failed to eval command: %w", err)
			}
			step.Command = value
		}

		for i, arg := range step.Args {
			value, err := stepContext.EvalString(arg, cmdutil.WithoutExpandEnv())
			if err != nil {
				return fmt.Errorf("failed to eval command args: %w", err)
			}
			step.Args[i] = value
		}
	}

	n.data.SetStep(step)
	n.cmdEvaluated.Store(true)
	return nil
}

func (n *SafeData) GetRetryCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.data.State.RetryCount
}

func (n *SafeData) SetRetriedAt(retriedAt time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.data.State.RetriedAt = retriedAt
}

func (n *SafeData) GetDoneCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.data.State.DoneCount
}

func (n *SafeData) GetExitCode() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.data.State.ExitCode
}

func (n *SafeData) SetExitCode(exitCode int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.data.State.ExitCode = exitCode
}

func (n *SafeData) ClearState() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.data.State = NodeState{}
}

func (n *SafeData) MarkError(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.data.State.Error = err
	n.data.State.Status = NodeStatusError
}

func (n *Node) Signal(ctx context.Context, sig os.Signal, allowOverride bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	status := n.data.Status()
	if status == NodeStatusRunning && n.cmd != nil {
		sigsig := sig
		if allowOverride && n.data.SignalOnStop() != "" {
			sigsig = unix.SignalNum(n.data.SignalOnStop())
		}
		logger.Info(ctx, "Sending signal", "signal", sigsig, "step", n.data.Name())
		if err := n.cmd.Kill(sigsig); err != nil {
			logger.Error(ctx, "Failed to send signal", "err", err, "step", n.data.Name())
		}
	}
	if status == NodeStatusRunning {
		n.data.SetStatus(NodeStatusCancel)
	}
}

func (n *Node) Cancel(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.data.Status()
	if status == NodeStatusRunning {
		n.data.SetStatus(NodeStatusCancel)
	}
	if n.cancelFunc != nil {
		logger.Info(ctx, "canceling node", "step", n.data.Name())
		n.cancelFunc()
	}
}

func (n *Node) SetupContextBeforeExec(ctx context.Context) context.Context {
	n.mu.RLock()
	defer n.mu.RUnlock()

	stepContext := digraph.GetStepContext(ctx)
	stepContext = stepContext.WithEnv(digraph.EnvKeyLogPath, n.data.Log())
	stepContext = stepContext.WithEnv(digraph.EnvKeyDAGStepLogPath, n.data.Log())

	return digraph.WithStepContext(ctx, stepContext)
}

func (n *Node) Setup(ctx context.Context, logDir string, requestID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Set the log file path
	startedAt := time.Now()
	safeName := fileutil.SafeName(n.data.Name())
	timestamp := startedAt.Format("20060102.15:04:05.000")
	postfix := stringutil.TruncString(requestID, 8)
	logFilename := fmt.Sprintf("%s.%s.%s.log", safeName, timestamp, postfix)
	if !fileutil.FileExists(logDir) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory %q: %w", logDir, err)
		}
	}

	logFile := filepath.Join(logDir, logFilename)
	n.data.Setup(ctx, logFile, startedAt)

	if err := n.outputs.setup(ctx, n.data.Data()); err != nil {
		n.data.SetError(err)
	}
	if err := n.setupRetryPolicy(ctx); err != nil {
		return fmt.Errorf("failed to setup retry policy: %w", err)
	}
	if err := n.setupScript(); err != nil {
		return fmt.Errorf("failed to setup script: %w", err)
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

	if n.script != nil {
		n.script.SilentRemove()
	}
	if lastErr != nil {
		n.data.SetError(lastErr)
	}

	return lastErr
}

func (n *SafeData) IncRetryCount() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.data.State.RetryCount++
}

func (n *SafeData) IncDoneCount() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.data.State.DoneCount++
}

var (
	ErrWorkingDirNotExist = fmt.Errorf("working directory does not exist")
)

func (n *Node) setupScript() (err error) {
	step := n.data.Step()
	if step.Script != "" {
		if len(step.Dir) > 0 && !fileutil.FileExists(step.Dir) {
			return ErrWorkingDirNotExist
		}

		file, err := os.CreateTemp(step.Dir, "dagu_script-")
		if err != nil {
			return fmt.Errorf("failed to create script file: %w", err)
		}
		defer func() {
			_ = file.Close()
		}()

		if _, err = file.WriteString(step.Script); err != nil {
			return fmt.Errorf("failed to write script to file: %w", err)
		}

		if err = file.Sync(); err != nil {
			return fmt.Errorf("failed to sync script file: %w", err)
		}

		n.script = &Script{name: file.Name()}
	}
	return err
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
	logFilename := n.outputs.LogFile()
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
	Limit    int
	Interval time.Duration
}

func (n *Node) setupRetryPolicy(ctx context.Context) error {
	var limit int
	var interval time.Duration

	step := n.data.Step()
	if step.RetryPolicy.Limit > 0 {
		limit = step.RetryPolicy.Limit
	}

	if step.RetryPolicy.Interval > 0 {
		interval = step.RetryPolicy.Interval
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
		Limit:    limit,
		Interval: interval,
	}

	return nil
}

type Script struct {
	name string
}

func (s *Script) Name() string {
	return s.name
}

func (s *Script) SilentRemove() {
	_ = os.Remove(s.name)
}

type OutputCoordinator struct {
	mu           sync.Mutex
	logFilename  string
	logFile      *os.File
	logWriter    *bufio.Writer
	stdoutFile   *os.File
	stdoutWriter *bufio.Writer
	stderrFile   *os.File
	stderrWriter *bufio.Writer
	outputWriter *os.File
	outputReader *os.File
}

func (oc *OutputCoordinator) LogFile() string {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	return oc.logFilename
}

func (oc *OutputCoordinator) lock() {
	oc.mu.Lock()
}

func (oc *OutputCoordinator) unlock() {
	oc.mu.Unlock()
}

func (oc *OutputCoordinator) setup(ctx context.Context, data NodeData) error {
	if err := oc.setupLog(ctx, data); err != nil {
		return err
	}
	if err := oc.setupStdout(ctx, data); err != nil {
		return err
	}
	return oc.setupStderr(ctx, data)
}

func (oc *OutputCoordinator) setupExecutorIO(_ context.Context, cmd executor.Executor, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var stdout io.Writer

	// Output to log only
	if oc.logWriter != nil {
		stdout = oc.logWriter
		cmd.SetStderr(stdout)
	}

	// Output to both log and stdout
	if oc.stdoutWriter != nil {
		stdout = io.MultiWriter(oc.logWriter, oc.stdoutWriter)
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

	if oc.stderrWriter != nil {
		cmd.SetStderr(oc.stderrWriter)
	} else {
		// If stderr output is not set, use stdout for stderr as well
		cmd.SetStderr(stdout)
	}

	return nil
}

func (oc *OutputCoordinator) closeResources(_ context.Context) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var lastErr error
	for _, w := range []*bufio.Writer{oc.logWriter, oc.stdoutWriter, oc.stderrWriter} {
		if w != nil {
			if err := w.Flush(); err != nil {
				lastErr = err
			}
		}
	}
	for _, f := range []*os.File{oc.logFile, oc.stdoutFile, oc.stderrFile} {
		if f != nil {
			if err := f.Sync(); err != nil {
				lastErr = err
			}
			_ = f.Close()
		}
	}
	return lastErr
}

func (oc *OutputCoordinator) setupStdout(ctx context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if data.Step.Stdout == "" {
		return nil
	}

	file, err := oc.setupFile(ctx, data.Step.Stdout, data)
	if err != nil {
		return fmt.Errorf("failed to setup stdout file: %w", err)
	}

	oc.stdoutFile = file
	oc.stdoutWriter = bufio.NewWriter(oc.stdoutFile)

	return nil
}

func (oc *OutputCoordinator) setupStderr(ctx context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if data.Step.Stderr == "" {
		return nil
	}

	file, err := oc.setupFile(ctx, data.Step.Stderr, data)
	if err != nil {
		return fmt.Errorf("failed to setup stderr file: %w", err)
	}

	oc.stderrFile = file
	oc.stderrWriter = bufio.NewWriter(oc.stderrFile)

	return nil
}

func (oc *OutputCoordinator) setupLog(_ context.Context, data NodeData) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	var err error
	oc.logFile, err = fileutil.OpenOrCreateFile(data.State.Log)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	oc.logWriter = bufio.NewWriter(oc.logFile)
	oc.logFilename = data.State.Log

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

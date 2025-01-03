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
	cmd, err := n.SetupExec(ctx)
	if err != nil {
		return err
	}

	n.setError(cmd.Run(ctx))

	if n.outputReader != nil && n.data.Step.Output != "" {
		if err := n.outputWriter.Close(); err != nil {
			logger.Error(ctx, "failed to close pipe writer", "err", err)
		}
		var buf bytes.Buffer
		// TODO: Error handling
		_, _ = io.Copy(&buf, n.outputReader)
		ret := strings.TrimSpace(buf.String())
		_ = os.Setenv(n.data.Step.Output, ret)

		if n.data.Step.OutputVariables == nil {
			n.data.Step.OutputVariables = &digraph.SyncMap{}
		}

		n.data.Step.OutputVariables.Store(
			n.data.Step.Output,
			fmt.Sprintf("%s=%s", n.data.Step.Output, ret),
		)
	}

	return n.data.State.Error
}

func (n *Node) Finish() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.FinishedAt = time.Now()
}

func (n *Node) SetupExec(ctx context.Context) (executor.Executor, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	ctx, fn := context.WithCancel(ctx)

	n.cancelFunc = fn

	if n.data.Step.CmdWithArgs != "" {
		cmd, args, err := cmdutil.SplitCommandWithEval(n.data.Step.CmdWithArgs)
		if err != nil {
			return nil, fmt.Errorf("failed to split command: %w", err)
		}
		n.data.Step.Command = cmd
		n.data.Step.Args = args
	}

	if n.data.Step.Command == "" {
		// If the command is empty, use the default shell as the command
		n.data.Step.Command = cmdutil.GetShellCommand(n.data.Step.Shell)
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
	if err := n.setupRetryPolicy(); err != nil {
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

func (n *Node) setupRetryPolicy() error {
	var retryPolicy retryPolicy

	if n.data.Step.RetryPolicy.Limit > 0 {
		retryPolicy.Limit = n.data.Step.RetryPolicy.Limit
	}
	if n.data.Step.RetryPolicy.Interval > 0 {
		retryPolicy.Interval = n.data.Step.RetryPolicy.Interval
	}
	// Evaluate the the configuration if it's configured as a string
	// e.g. environment variable or command substitution
	if n.data.Step.RetryPolicy.LimitStr != "" {
		v, err := cmdutil.EvalIntString(n.data.Step.RetryPolicy.LimitStr)
		if err != nil {
			return fmt.Errorf("failed to substitute retry limit %q: %w", n.data.Step.RetryPolicy.LimitStr, err)
		}
		retryPolicy.Limit = v
	}
	if n.data.Step.RetryPolicy.IntervalSecStr != "" {
		v, err := cmdutil.EvalIntString(n.data.Step.RetryPolicy.IntervalSecStr)
		if err != nil {
			return fmt.Errorf("failed to substitute retry interval %q: %w", n.data.Step.RetryPolicy.IntervalSecStr, err)
		}
		retryPolicy.Interval = time.Duration(v) * time.Second
	}
	n.retryPolicy = retryPolicy
	return nil
}

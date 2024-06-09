package scheduler

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/executor"
	"github.com/dagu-dev/dagu/internal/util"
	"golang.org/x/sys/unix"
)

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

func NewNode(step dag.Step, state NodeState) *Node {
	return &Node{step: step, NodeState: state}
}

// Node is a node in a DAG. It executes a command.
type Node struct {
	step dag.Step
	NodeState

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
}

// NodeState is the state of a node.
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

func (n *Node) finish() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.FinishedAt = time.Now()
}

func (n *Node) SetError(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Error = err
}

func (n *Node) State() NodeState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.NodeState
}

// Execute runs the command synchronously and returns error if any.
func (n *Node) Execute(ctx context.Context) error {
	cmd, err := n.setupExec(ctx)
	if err != nil {
		return err
	}
	n.SetError(cmd.Run())
	if n.outputReader != nil && n.step.Output != "" {
		util.LogErr("close pipe writer", n.outputWriter.Close())
		var buf bytes.Buffer
		// TODO: Error handling
		_, _ = io.Copy(&buf, n.outputReader)
		ret := strings.TrimSpace(buf.String())
		_ = os.Setenv(n.step.Output, ret)
		n.step.OutputVariables.Store(n.step.Output, fmt.Sprintf("%s=%s", n.step.Output, ret))
	}

	return n.Error
}

func (n *Node) setupExec(ctx context.Context) (executor.Executor, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	ctx, fn := context.WithCancel(ctx)

	n.cancelFunc = fn

	if n.step.CmdWithArgs != "" {
		n.step.Command, n.step.Args = util.SplitCommand(n.step.CmdWithArgs, true)
	}

	if n.scriptFile != nil {
		var args []string
		args = append(args, n.step.Args...)
		n.step.Args = append(args, n.scriptFile.Name())
	}

	cmd, err := executor.CreateExecutor(ctx, n.step)
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

	if n.step.Output != "" {
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

func (n *Node) Step() dag.Step {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.step
}

func (n *Node) getRetryCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.RetryCount
}

func (n *Node) setRetriedAt(retriedAt time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.RetriedAt = retriedAt
}

func (n *Node) getRetriedAt() time.Time {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.RetriedAt
}

func (n *Node) getDoneCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.DoneCount
}

func (n *Node) clearState() {
	n.NodeState = NodeState{}
}

func (n *Node) setStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Status = status
}

func (n *Node) setErr(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Error = err
	n.Status = NodeStatusError
}

func (n *Node) signal(sig os.Signal, allowOverride bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.Status
	if status == NodeStatusRunning && n.cmd != nil {
		sigsig := sig
		if allowOverride && n.step.SignalOnStop != "" {
			sigsig = unix.SignalNum(n.step.SignalOnStop)
		}
		log.Printf("Sending %s signal to %s", sigsig, n.step.Name)
		util.LogErr("sending signal", n.cmd.Kill(sigsig))
	}
	if status == NodeStatusRunning {
		n.Status = NodeStatusCancel
	}
}

func (n *Node) cancel() {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.Status
	if status == NodeStatusRunning {
		n.Status = NodeStatusCancel
	}
	if n.cancelFunc != nil {
		log.Printf("canceling node: %s", n.step.Name)
		n.cancelFunc()
	}
}

func (n *Node) setup(logDir string, requestId string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.StartedAt = time.Now()
	n.Log = filepath.Join(logDir, fmt.Sprintf("%s.%s.%s.log",
		util.ValidFilename(n.step.Name, "_"),
		n.StartedAt.Format("20060102.15:04:05.000"),
		util.TruncString(requestId, 8),
	))
	for _, fn := range []func() error{
		n.setupLog,
		n.setupStdout,
		n.setupStderr,
		n.setupScript,
	} {
		if err := fn(); err != nil {
			n.Error = err
			return err
		}
	}
	return nil
}

func (n *Node) setupScript() (err error) {
	if n.step.Script != "" {
		if len(n.step.Dir) > 0 && !util.FileExists(n.step.Dir) {
			return fmt.Errorf("directory %q does not exist", n.step.Dir)
		}
		n.scriptFile, _ = os.CreateTemp(n.step.Dir, "dagu_script-")
		if _, err = n.scriptFile.WriteString(n.step.Script); err != nil {
			return
		}
		defer func() {
			_ = n.scriptFile.Close()
		}()
		err = n.scriptFile.Sync()
	}
	return err
}

func (n *Node) setupStdout() error {
	if n.step.Stdout != "" {
		f := n.step.Stdout
		if !filepath.IsAbs(f) {
			f = filepath.Join(n.step.Dir, f)
		}
		var err error
		n.stdoutFile, err = util.OpenOrCreateFile(f)
		if err != nil {
			n.Error = err
			return err
		}
		n.stdoutWriter = bufio.NewWriter(n.stdoutFile)
	}
	return nil
}

func (n *Node) setupStderr() error {
	if n.step.Stderr != "" {
		f := n.step.Stderr
		if !filepath.IsAbs(f) {
			f = filepath.Join(n.step.Dir, f)
		}
		var err error
		n.stderrFile, err = util.OpenOrCreateFile(f)
		if err != nil {
			n.Error = err
			return err
		}
		n.stderrWriter = bufio.NewWriter(n.stderrFile)
	}
	return nil
}

func (n *Node) setupLog() error {
	if n.Log == "" {
		return nil
	}
	n.logLock.Lock()
	defer n.logLock.Unlock()
	var err error
	n.logFile, err = util.OpenOrCreateFile(n.Log)
	if err != nil {
		n.Error = err
		return err
	}
	n.logWriter = bufio.NewWriter(n.logFile)
	return nil
}

func (n *Node) teardown() error {
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
		n.Error = lastErr
	}
	return lastErr
}

func (n *Node) incRetryCount() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.RetryCount++
}

func (n *Node) incDoneCount() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.DoneCount++
}

var nextNodeId = 1

func (n *Node) init() {
	if n.id != 0 {
		return
	}
	n.id = nextNodeId
	nextNodeId++
	if n.step.Variables == nil {
		n.step.Variables = []string{}
	}
	if n.step.Variables == nil {
		n.step.Variables = []string{}
	}
	if n.step.Preconditions == nil {
		n.step.Preconditions = []*dag.Condition{}
	}
}

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

	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/executor"
	"github.com/yohamta/dagu/internal/utils"
	"golang.org/x/sys/unix"
)

type NodeStatus int

const (
	NodeStatus_None NodeStatus = iota
	NodeStatus_Running
	NodeStatus_Error
	NodeStatus_Cancel
	NodeStatus_Success
	NodeStatus_Skipped
)

func (s NodeStatus) String() string {
	switch s {
	case NodeStatus_Running:
		return "running"
	case NodeStatus_Error:
		return "failed"
	case NodeStatus_Cancel:
		return "canceled"
	case NodeStatus_Success:
		return "finished"
	case NodeStatus_Skipped:
		return "skipped"
	case NodeStatus_None:
		fallthrough
	default:
		return "not started"
	}
}

// Node is a node in a DAG. It executes a command.
type Node struct {
	*dag.Step
	NodeState

	id           int
	mu           sync.RWMutex
	cmd          executor.Executor
	cancelFunc   func()
	logFile      *os.File
	logWriter    *bufio.Writer
	stdoutFile   *os.File
	stdoutWriter *bufio.Writer
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

// Execute runs the command synchronously and returns error if any.
func (n *Node) Execute() error {
	ctx, fn := context.WithCancel(context.Background())
	n.cancelFunc = fn

	if n.CmdWithArgs != "" {
		n.Command, n.Args = utils.SplitCommand(n.CmdWithArgs, true)
	}

	if n.scriptFile != nil {
		args := []string{}
		args = append(args, n.Args...)
		n.Args = append(args, n.scriptFile.Name())
	}

	cmd, err := executor.CreateExecutor(ctx, n.Step)
	if err != nil {
		return err
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

	if n.Output != "" {
		var err error
		if n.outputReader, n.outputWriter, err = os.Pipe(); err != nil {
			return err
		}
		stdout = io.MultiWriter(stdout, n.outputWriter)
	}

	cmd.SetStdout(stdout)

	n.Error = cmd.Run()

	if n.outputReader != nil && n.Output != "" {
		utils.LogErr("close pipe writer", n.outputWriter.Close())
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, n.outputReader)
		ret := strings.TrimSpace(buf.String())
		os.Setenv(n.Output, ret)
		n.OutputVariables.Store(n.Output, fmt.Sprintf("%s=%s", n.Output, ret))
	}

	return n.Error
}

// ReadStatus reads the status of a node.
func (n *Node) ReadStatus() NodeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()
	ret := n.Status
	return ret
}

func (n *Node) ReadRetryCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.RetryCount
}

func (n *Node) SetRetriedAt(retriedAt time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.RetriedAt = retriedAt
}

func (n *Node) ReadRetriedAt() time.Time {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.RetriedAt
}

func (n *Node) ReadDoneCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.DoneCount
}

func (n *Node) clearState() {
	n.NodeState = NodeState{}
}

func (n *Node) updateStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Status = status
}

func (n *Node) signal(sig os.Signal, allowOverride bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.Status
	if status == NodeStatus_Running && n.cmd != nil {
		sigsig := sig
		if allowOverride && n.Step.SignalOnStep != "" {
			sigsig = unix.SignalNum(n.Step.SignalOnStep)
		}
		log.Printf("Sending %s signal to %s", sigsig, n.Name)
		utils.LogErr("sending signal", n.cmd.Kill(sigsig))
	}
	if status == NodeStatus_Running {
		n.Status = NodeStatus_Cancel
	}
}

func (n *Node) cancel() {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.Status
	if status == NodeStatus_Running {
		n.Status = NodeStatus_Cancel
	}
	if n.cancelFunc != nil {
		log.Printf("canceling node: %s", n.Step.Name)
		n.cancelFunc()
	}
}

func (n *Node) setup(logDir string, requestId string) error {
	n.StartedAt = time.Now()
	n.Log = filepath.Join(logDir, fmt.Sprintf("%s.%s.%s.log",
		utils.ValidFilename(n.Name, "_"),
		n.StartedAt.Format("20060102.15:04:05.000"),
		utils.TruncString(requestId, 8),
	))
	setup := []func() error{
		n.setupLog,
		n.setupStdout,
		n.setupScript,
	}
	for _, fn := range setup {
		err := fn()
		if err != nil {
			n.Error = err
			return err
		}
	}
	return nil
}

func (n *Node) setupScript() (err error) {
	if n.Script != "" {
		n.scriptFile, _ = os.CreateTemp(n.Dir, "dagu_script-")
		if _, err = n.scriptFile.WriteString(n.Script); err != nil {
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
	if n.Stdout != "" {
		f := n.Stdout
		if !filepath.IsAbs(f) {
			f = filepath.Join(n.Dir, f)
		}
		var err error
		n.stdoutFile, err = utils.OpenOrCreateFile(f)
		if err != nil {
			n.Error = err
			return err
		}
		n.stdoutWriter = bufio.NewWriter(n.stdoutFile)
	}
	return nil
}

func (n *Node) setupLog() error {
	if n.Log == "" {
		return nil
	}
	var err error
	n.logFile, err = utils.OpenOrCreateFile(n.Log)
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
	n.done = true
	var lastErr error = nil
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

var nextNodeId int = 1

func (n *Node) init() {
	if n.id != 0 {
		return
	}
	n.id = nextNodeId
	nextNodeId++
	if n.Variables == nil {
		n.Variables = []string{}
	}
	if n.Variables == nil {
		n.Variables = []string{}
	}
	if n.Preconditions == nil {
		n.Preconditions = []*dag.Condition{}
	}
}

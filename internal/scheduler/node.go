package scheduler

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/utils"
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

type Node struct {
	*config.Step
	NodeState
	id           int
	mu           sync.RWMutex
	cmd          *exec.Cmd
	cancelFunc   func()
	logFile      *os.File
	logWriter    *bufio.Writer
	stdoutFile   *os.File
	stdoutWriter *bufio.Writer
}

type NodeState struct {
	Status     NodeStatus
	Log        string
	StartedAt  time.Time
	FinishedAt time.Time
	RetryCount int
	DoneCount  int
	Error      error
}

func (n *Node) Execute() error {
	ctx, fn := context.WithCancel(context.Background())
	n.cancelFunc = fn
	n.cmd = exec.CommandContext(ctx, n.Command, n.Args...)
	cmd := n.cmd
	cmd.Dir = n.Dir
	cmd.Env = append(cmd.Env, n.Variables...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	if n.logWriter != nil {
		cmd.Stdout = n.logWriter
		cmd.Stderr = n.logWriter
	}

	if n.stdoutWriter != nil {
		cmd.Stdout = io.MultiWriter(n.logWriter, n.stdoutWriter)
	}

	n.Error = cmd.Run()
	return n.Error
}

func (n *Node) clearState() {
	n.NodeState = NodeState{}
}

func (n *Node) ReadStatus() NodeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()
	ret := n.Status
	return ret
}

func (n *Node) updateStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Status = status
}

func (n *Node) signal(sig os.Signal) {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.Status
	if status == NodeStatus_Running && n.cmd != nil {
		log.Printf("Sending %s signal to %s", sig, n.Name)
		utils.LogIgnoreErr("sending signal", syscall.Kill(-n.cmd.Process.Pid, sig.(syscall.Signal)))
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
	if err := n.openLog(); err != nil {
		n.Error = err
		return err
	}
	if n.Stdout != "" {
		if err := n.openStdout(); err != nil {
			n.Error = err
			return err
		}
	}
	return nil
}

func (n *Node) openStdout() error {
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
	return nil
}

func (n *Node) openLog() error {
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
	var lastErr error = nil
	if n.logWriter != nil {
		lastErr = n.logWriter.Flush()
	}
	if n.logFile != nil {
		if err := n.logFile.Close(); err != nil {
			lastErr = err
		}
	}
	if n.stdoutWriter != nil {
		lastErr = n.stdoutWriter.Flush()
	}
	if n.stdoutFile != nil {
		if err := n.stdoutFile.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (n *Node) ReadRetryCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.RetryCount
}

func (n *Node) ReadDoneCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.DoneCount
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
		n.Preconditions = []*config.Condition{}
	}
}

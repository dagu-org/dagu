// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

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

	"golang.org/x/sys/unix"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/util"
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

func (n *Node) SetError(err error) {
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
	dagCtx, err := digraph.GetContext(ctx)
	if err != nil {
		return err
	}
	// Add the log path to the environment
	dagCtx = dagCtx.WithEnv(digraph.Env{
		Key:   digraph.EnvKeyLogPath,
		Value: n.data.State.Log,
	})
	ctx = digraph.WithDagContext(ctx, dagCtx)
	cmd, err := n.setupExec(ctx)
	if err != nil {
		return err
	}
	n.SetError(cmd.Run())
	if n.outputReader != nil && n.data.Step.Output != "" {
		util.LogErr("close pipe writer", n.outputWriter.Close())
		var buf bytes.Buffer
		// TODO: Error handling
		_, _ = io.Copy(&buf, n.outputReader)
		ret := strings.TrimSpace(buf.String())
		_ = os.Setenv(n.data.Step.Output, ret)
		n.data.Step.OutputVariables.Store(
			n.data.Step.Output,
			fmt.Sprintf("%s=%s", n.data.Step.Output, ret),
		)
	}

	return n.data.State.Error
}

func (n *Node) finish() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.FinishedAt = time.Now()
}

func (n *Node) setupExec(ctx context.Context) (executor.Executor, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	ctx, fn := context.WithCancel(ctx)

	n.cancelFunc = fn

	if n.data.Step.CmdWithArgs != "" {
		n.data.Step.Command, n.data.Step.Args =
			util.SplitCommandWithParse(n.data.Step.CmdWithArgs)
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

func (n *Node) getRetryCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data.State.RetryCount
}

func (n *Node) setRetriedAt(retriedAt time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.RetriedAt = retriedAt
}

func (n *Node) getDoneCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.data.State.DoneCount
}

func (n *Node) clearState() {
	n.data.State = NodeState{}
}

func (n *Node) setStatus(status NodeStatus) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.Status = status
}

func (n *Node) setErr(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.Error = err
	n.data.State.Status = NodeStatusError
}

func (n *Node) signal(sig os.Signal, allowOverride bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.data.State.Status
	if status == NodeStatusRunning && n.cmd != nil {
		sigsig := sig
		if allowOverride && n.data.Step.SignalOnStop != "" {
			sigsig = unix.SignalNum(n.data.Step.SignalOnStop)
		}
		log.Printf("Sending %s signal to %s", sigsig, n.data.Step.Name)
		util.LogErr("sending signal", n.cmd.Kill(sigsig))
	}
	if status == NodeStatusRunning {
		n.data.State.Status = NodeStatusCancel
	}
}

func (n *Node) cancel() {
	n.mu.Lock()
	defer n.mu.Unlock()
	status := n.data.State.Status
	if status == NodeStatusRunning {
		n.data.State.Status = NodeStatusCancel
	}
	if n.cancelFunc != nil {
		log.Printf("canceling node: %s", n.data.Step.Name)
		n.cancelFunc()
	}
}

func (n *Node) setup(logDir string, requestID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Set the log file path
	n.data.State.StartedAt = time.Now()
	n.data.State.Log = filepath.Join(logDir, fmt.Sprintf("%s.%s.%s.log",
		fileutil.SafeName(n.data.Step.Name),
		n.data.State.StartedAt.Format("20060102.15:04:05.000"),
		util.TruncString(requestID, 8),
	))

	// Replace the special environment variables in the command
	// Why this is necessary:
	// 1. We need to expand the environment variables when setup the node.
	// 2. The environment variables need to be set in the current process.
	// 3. But since the values of the log path are different for each node,
	//    we need to replace the name differently for each node.
	envKeyLogPath := fmt.Sprintf("STEP_%d_DAG_EXECUTION_LOG_PATH", n.id)
	if err := os.Setenv(envKeyLogPath, n.data.State.Log); err != nil {
		return err
	}

	// Expand environment variables in the step
	n.data.Step.CmdWithArgs = strings.ReplaceAll(
		n.data.Step.CmdWithArgs,
		digraph.EnvKeyLogPath,
		envKeyLogPath,
	)

	n.data.Step.Stdout = os.ExpandEnv(n.data.Step.Stdout)
	n.data.Step.Stderr = os.ExpandEnv(n.data.Step.Stderr)
	n.data.Step.Dir = os.ExpandEnv(n.data.Step.Dir)

	if err := n.setupLog(); err != nil {
		return err
	}
	if err := n.setupStdout(); err != nil {
		return err
	}
	if err := n.setupStderr(); err != nil {
		return err
	}
	return n.setupScript()
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
		n.data.State.Error = lastErr
	}
	return lastErr
}

func (n *Node) incRetryCount() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.RetryCount++
}

func (n *Node) incDoneCount() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.data.State.DoneCount++
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

func (n *Node) init() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.id != 0 {
		return
	}
	n.id = getNextNodeID()

	if n.data.Step.Variables == nil {
		n.data.Step.Variables = []string{}
	}
	if n.data.Step.Variables == nil {
		n.data.Step.Variables = []string{}
	}
	if n.data.Step.Preconditions == nil {
		n.data.Step.Preconditions = []digraph.Condition{}
	}
}

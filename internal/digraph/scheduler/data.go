package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// SafeData is a thread-safe wrapper around NodeData.
type SafeData struct {
	mu    sync.RWMutex
	inner NodeData
}

// NodeData represents the data of a node.
type NodeData struct {
	Step  digraph.Step
	State NodeState
}

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

func newSafeData(data NodeData) SafeData {
	return SafeData{inner: data}
}

func (s *SafeData) ResetError() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.State.Error = nil
	s.inner.State.ExitCode = 0
}

func (s *SafeData) Args() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	args := make([]string, len(s.inner.Step.Args))
	copy(args, s.inner.Step.Args)
	return args
}

func (s *SafeData) SetArgs(args []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.Step.Args = args
}

func (s *SafeData) Step() digraph.Step {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.Step
}

func (s *SafeData) SetStep(step digraph.Step) {
	// TODO: refactor to avoid modifying the step
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.Step = step
}

func (s *SafeData) Data() NodeData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner
}

func (s *SafeData) Setup(ctx context.Context, logFile string, startedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.State.Log = logFile
	s.inner.State.StartedAt = startedAt

	stepContext := digraph.GetStepContext(ctx)

	// Evaluate the stdout and stderr fields
	stdout, err := stepContext.EvalString(s.inner.Step.Stdout)
	if err != nil {
		return fmt.Errorf("failed to evaluate stdout field: %w", err)
	}
	s.inner.Step.Stdout = stdout

	stderr, err := stepContext.EvalString(s.inner.Step.Stderr)
	if err != nil {
		return fmt.Errorf("failed to evaluate stderr field: %w", err)
	}
	s.inner.Step.Stderr = stderr

	// Evaluate the dir field
	dir, err := stepContext.EvalString(s.inner.Step.Dir)
	if err != nil {
		return fmt.Errorf("failed to evaluate dir field: %w", err)
	}
	s.inner.Step.Dir = dir

	return nil
}

func (s *SafeData) State() NodeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.State
}

func (s *SafeData) Status() NodeStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.State.Status
}

func (s *SafeData) SetStatus(status NodeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.State.Status = status
}

func (s *SafeData) ContinueOn() digraph.ContinueOn {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.Step.ContinueOn
}

func (s *SafeData) Log() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.State.Log
}

func (s *SafeData) SignalOnStop() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.Step.SignalOnStop
}

func (s *SafeData) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.Step.Name
}

func (s *SafeData) Error() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.State.Error
}

func (s *SafeData) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.State.Error = err
}

func (s *SafeData) ClearVariable(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inner.Step.OutputVariables == nil {
		return
	}

	s.inner.Step.OutputVariables.Delete(key)
}

func (s *SafeData) MatchExitCode(exitCodes []int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, code := range exitCodes {
		if code == s.inner.State.ExitCode {
			return true
		}
	}
	return false
}

func (n *SafeData) getVariable(key string) (stringutil.KeyValue, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.inner.Step.OutputVariables == nil {
		return "", false
	}

	v, ok := n.inner.Step.OutputVariables.Load(key)
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
	if n.inner.Step.OutputVariables == nil {
		n.inner.Step.OutputVariables = &digraph.SyncMap{}
	}
	n.inner.Step.OutputVariables.Store(key, stringutil.NewKeyValue(key, strconv.FormatBool(value)).String())
}

func (n *SafeData) setVariable(key, value string) {
	if n.inner.Step.OutputVariables == nil {
		n.inner.Step.OutputVariables = &digraph.SyncMap{}
	}
	n.inner.Step.OutputVariables.Store(key, stringutil.NewKeyValue(key, value).String())
}

func (n *SafeData) Finish() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.FinishedAt = time.Now()
}

func (n *SafeData) IncRetryCount() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.RetryCount++
}

func (n *SafeData) GetRetryCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.inner.State.RetryCount
}

func (n *SafeData) SetRetriedAt(retriedAt time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.RetriedAt = retriedAt
}

func (n *SafeData) IncDoneCount() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.DoneCount++
}

func (n *SafeData) GetDoneCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.inner.State.DoneCount
}

func (n *SafeData) GetExitCode() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.inner.State.ExitCode
}

func (n *SafeData) SetExitCode(exitCode int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.ExitCode = exitCode
}

func (n *SafeData) ClearState() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State = NodeState{}
}

func (n *SafeData) MarkError(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.Error = err
	n.inner.State.Status = NodeStatusError
}

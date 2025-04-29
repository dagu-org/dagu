package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/google/uuid"
)

// Data is a thread-safe wrapper around NodeData.
type Data struct {
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
	RequestID  string
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

func newSafeData(data NodeData) Data {
	return Data{inner: data}
}

func (s *Data) ResetError() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.State.Error = nil
	s.inner.State.ExitCode = 0
}

func (s *Data) Args() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	args := make([]string, len(s.inner.Step.Args))
	copy(args, s.inner.Step.Args)
	return args
}

func (s *Data) SetArgs(args []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.Step.Args = args
}

func (s *Data) Step() digraph.Step {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.Step
}

func (s *Data) SetStep(step digraph.Step) {
	// TODO: refactor to avoid modifying the step
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.Step = step
}

func (s *Data) Data() NodeData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner
}

func (s *Data) RequestID() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inner.State.RequestID == "" {
		reqID, err := generateRequestID()
		if err != nil {
			return "", fmt.Errorf("failed to generate request ID: %w", err)
		}
		s.inner.State.RequestID = reqID
	}

	return s.inner.State.RequestID, nil
}

func (s *Data) Setup(ctx context.Context, logFile string, startedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.State.Log = logFile
	s.inner.State.StartedAt = startedAt

	c := digraph.GetExecContext(ctx)

	// Evaluate the stdout and stderr fields
	stdout, err := c.EvalString(ctx, s.inner.Step.Stdout)
	if err != nil {
		return fmt.Errorf("failed to evaluate stdout field: %w", err)
	}
	s.inner.Step.Stdout = stdout

	stderr, err := c.EvalString(ctx, s.inner.Step.Stderr)
	if err != nil {
		return fmt.Errorf("failed to evaluate stderr field: %w", err)
	}
	s.inner.Step.Stderr = stderr

	// Evaluate the dir field
	dir, err := c.EvalString(ctx, s.inner.Step.Dir)
	if err != nil {
		return fmt.Errorf("failed to evaluate dir field: %w", err)
	}
	s.inner.Step.Dir = dir

	return nil
}

func (s *Data) State() NodeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.State
}

func (s *Data) Status() NodeStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.State.Status
}

func (s *Data) SetStatus(status NodeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.State.Status = status
}

func (s *Data) ContinueOn() digraph.ContinueOn {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.Step.ContinueOn
}

func (s *Data) Log() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.State.Log
}

func (s *Data) SignalOnStop() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.Step.SignalOnStop
}

func (s *Data) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.Step.Name
}

func (s *Data) Error() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.inner.State.Error
}

func (s *Data) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inner.State.Error = err
}

func (s *Data) ClearVariable(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inner.Step.OutputVariables == nil {
		return
	}

	s.inner.Step.OutputVariables.Delete(key)
}

func (s *Data) MatchExitCode(exitCodes []int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, code := range exitCodes {
		if code == s.inner.State.ExitCode {
			return true
		}
	}
	return false
}

func (n *Data) getVariable(key string) (stringutil.KeyValue, bool) {
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

func (n *Data) getBoolVariable(key string) (bool, bool) {
	v, ok := n.getVariable(key)
	if !ok {
		return false, false
	}

	return v.Bool(), true
}

func (n *Data) setBoolVariable(key string, value bool) {
	if n.inner.Step.OutputVariables == nil {
		n.inner.Step.OutputVariables = &digraph.SyncMap{}
	}
	n.inner.Step.OutputVariables.Store(key, stringutil.NewKeyValue(key, strconv.FormatBool(value)).String())
}

func (n *Data) setVariable(key, value string) {
	if n.inner.Step.OutputVariables == nil {
		n.inner.Step.OutputVariables = &digraph.SyncMap{}
	}
	n.inner.Step.OutputVariables.Store(key, stringutil.NewKeyValue(key, value).String())
}

func (n *Data) Finish() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.FinishedAt = time.Now()
}

func (n *Data) IncRetryCount() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.RetryCount++
}

func (n *Data) GetRetryCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.inner.State.RetryCount
}

func (n *Data) SetRetriedAt(retriedAt time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.RetriedAt = retriedAt
}

func (n *Data) IncDoneCount() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.DoneCount++
}

func (n *Data) GetDoneCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.inner.State.DoneCount
}

func (n *Data) GetExitCode() int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.inner.State.ExitCode
}

func (n *Data) SetExitCode(exitCode int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.ExitCode = exitCode
}

func (n *Data) ClearState() {
	n.mu.Lock()
	defer n.mu.Unlock()

	// requestID needs to be retained for sub-DAGs
	requestID := n.inner.State.RequestID
	n.inner.State = NodeState{}
	n.inner.State.RequestID = requestID
}

func (n *Data) MarkError(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.inner.State.Error = err
	n.inner.State.Status = NodeStatusError
}

// generateRequestID generates a new request ID.
// For simplicity, we use UUIDs as request IDs.
func generateRequestID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

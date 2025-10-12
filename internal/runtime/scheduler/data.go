package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/maputil"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/status"
)

// Data is a thread-safe wrapper around NodeData.
type Data struct {
	mu    sync.RWMutex
	inner NodeData
}

// NodeData represents the data of a node.
type NodeData struct {
	Step  core.Step
	State NodeState
}

type NodeState struct {
	// Status represents the state of the node.
	Status status.NodeStatus
	// Stdout is the log file path from the node.
	Stdout string
	// Stderr is the log file path for the error log (stderr).
	Stderr string
	// StartedAt is the time when the node started.
	StartedAt time.Time
	// FinishedAt is the time when the node finished.
	FinishedAt time.Time
	// RetryCount is the number of retries happened based on the retry policy.
	RetryCount int
	// RetriedAt is the time when the node was retried last time.
	RetriedAt time.Time
	// DoneCount is the number of times the node was executed.
	DoneCount int
	// Repeated is true if the node is a repeated step.
	// This is used to generate unique run IDs for repeated steps in case the node
	// runs nested DAGs.
	Repeated bool
	// Error is the error that the executor encountered.
	Error error
	// ExitCode is the exit code that the command exited with.
	// It only makes sense when the node is a command executor.
	ExitCode int
	// Parallel contains the evaluated parallel execution state for the node.
	// This is populated when a step has parallel configuration and tracks
	// all the items that need to be executed in parallel.
	*Parallel
	// Children stores the child dag-runs.
	Children []ChildDAGRun
	// ChildrenRepeated stores the repeated child dag-runs.
	ChildrenRepeated []ChildDAGRun
	// OutputVariables stores the output variables for the following steps.
	// It only contains the local output variables.
	OutputVariables *maputil.SyncMap
}

// Parallel represents the evaluated parallel execution configuration for a node.
// It contains the expanded list of items to be processed in parallel.
type Parallel struct {
	// Items contains all the parallel items to be executed.
	// Each item will result in a separate child DAG run.
	Items []ParallelItem
}

// ParallelItem represents a single item in a parallel execution.
// It combines the item data with a unique identifier for tracking.
type ParallelItem struct {
	// Item contains the actual data for this parallel execution.
	// It can be either a simple value or a map of parameters from core.ParallelItem.
	Item core.ParallelItem
}

// ChildDAGRun represents a child DAG execution within a parent DAG.
// Each child DAG run has a deterministic ID based on its parameters to ensure idempotency.
type ChildDAGRun struct {
	// DAGRunID is the unique identifier for the child dag-run.
	// It is generated as a base58-encoded SHA-256 hash of the string:
	// "<parent-dag-run-id>:<step-name>:<deterministic-json-params>"
	//
	// This deterministic ID generation ensures:
	// - Same parameters always produce the same child DAG run ID
	// - Retries reuse existing child DAG runs instead of creating duplicates
	// - Each step's children are namespaced by step name to prevent collisions
	//
	// The params are encoded as deterministic JSON (sorted keys) before hashing.
	// Example input: "abc123:process-regions:{"REGION":"us-east-1","VERSION":"1.0.0"}"
	// Example output: "5Kd3NBUAdUnhyzenEwVLy9pBKxSwXvE9FMPyR4UKZvpe"
	DAGRunID string
	// Params contains the raw parameters passed to the child DAG run.
	// This can be:
	// - A simple string: "param1 param2"
	// - Key-value pairs: "KEY1=value1 KEY2=value2"
	// - Raw JSON: '{"region": "us-east-1", "config": {"timeout": 30}}'
	// The exact format depends on how the DAG expects to receive parameters.
	Params string
}

func newSafeData(data NodeData) Data {
	return Data{inner: data}
}

func (d *Data) ResetError() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.Error = nil
	d.inner.State.ExitCode = 0
}

func (d *Data) SetExecutorConfig(cfg core.ExecutorConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.Step.ExecutorConfig = cfg
}

func (d *Data) SetChildDAG(childDAG core.ChildDAG) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.Step.ChildDAG = &childDAG
}

func (d *Data) Args() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	args := make([]string, len(d.inner.Step.Args))
	copy(args, d.inner.Step.Args)
	return args
}

func (d *Data) SetArgs(args []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.Step.Args = args
}

func (d *Data) Step() core.Step {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.Step
}

func (d *Data) SetScript(script string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.Step.Script = script
}

func (s *Data) SetStep(step core.Step) {
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

// SetChildRuns sets the children of the node.
func (d *Data) SetChildRuns(children []ChildDAGRun) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Clear the existing children and set the new ones
	d.inner.State.Children = make([]ChildDAGRun, len(children))
	copy(d.inner.State.Children, children)
}

// AddChildRunsRepeated adds the repeated child runs to the node.
func (d *Data) AddChildRunsRepeated(child ...ChildDAGRun) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.ChildrenRepeated = append(d.inner.State.ChildrenRepeated, child...)
}

func (d *Data) Setup(ctx context.Context, logFile string, startedAt time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.Stdout = logFile + ".out"
	d.inner.State.Stderr = logFile + ".err"
	d.inner.State.StartedAt = startedAt

	env := core.GetEnv(ctx)

	// Evaluate the stdout field
	stdout, err := env.EvalString(ctx, d.inner.Step.Stdout)
	if err != nil {
		return fmt.Errorf("failed to evaluate stdout field: %w", err)
	}
	d.inner.Step.Stdout = stdout

	// Evaluate the stderr field
	stderr, err := env.EvalString(ctx, d.inner.Step.Stderr)
	if err != nil {
		return fmt.Errorf("failed to evaluate stderr field: %w", err)
	}
	d.inner.Step.Stderr = stderr

	return nil
}

func (d *Data) State() NodeState {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State
}

func (d *Data) Status() status.NodeStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State.Status
}

func (d *Data) SetStatus(s status.NodeStatus) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.Status = s
}

func (d *Data) ContinueOn() core.ContinueOn {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.Step.ContinueOn
}

func (d *Data) GetStdout() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State.Stdout
}

func (d *Data) GetStderr() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State.Stderr
}

func (d *Data) SignalOnStop() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.Step.SignalOnStop
}

func (d *Data) Name() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.Step.Name
}

func (d *Data) Error() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State.Error
}

func (d *Data) SetError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.Error = err
}

func (d *Data) ClearVariable(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.inner.State.OutputVariables == nil {
		return
	}

	d.inner.State.OutputVariables.Delete(key)
}

func (d *Data) MatchExitCode(exitCodes []int) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, code := range exitCodes {
		if code == d.inner.State.ExitCode {
			return true
		}
	}
	return false
}

func (d *Data) getVariable(key string) (stringutil.KeyValue, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.inner.State.OutputVariables == nil {
		return "", false
	}

	v, ok := d.inner.State.OutputVariables.Load(key)
	if !ok {
		return "", false
	}

	return stringutil.KeyValue(v.(string)), true
}

func (d *Data) getBoolVariable(key string) (bool, bool) {
	v, ok := d.getVariable(key)
	if !ok {
		return false, false
	}

	return v.Bool(), true
}

func (d *Data) setBoolVariable(key string, value bool) {

	if d.inner.State.OutputVariables == nil {
		d.mu.Lock()
		d.inner.State.OutputVariables = &maputil.SyncMap{}
		d.mu.Unlock()
	}
	d.inner.State.OutputVariables.Store(key, stringutil.NewKeyValue(key, strconv.FormatBool(value)).String())
}

func (d *Data) setVariable(key, value string) {
	if d.inner.State.OutputVariables == nil {
		d.mu.Lock()
		d.inner.State.OutputVariables = &maputil.SyncMap{}
		d.mu.Unlock()
	}
	d.inner.State.OutputVariables.Store(key, stringutil.NewKeyValue(key, value).String())
}

func (d *Data) Finish() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.FinishedAt = time.Now()
}

func (d *Data) IncRetryCount() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.RetryCount++
}

func (d *Data) GetRetryCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State.RetryCount
}

func (d *Data) SetRetriedAt(retriedAt time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.RetriedAt = retriedAt
}

func (d *Data) IncDoneCount() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.DoneCount++
}

func (d *Data) SetRepeated(repeated bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.Repeated = repeated
}

func (d *Data) IsRepeated() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State.Repeated
}

func (d *Data) GetDoneCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State.DoneCount
}

func (d *Data) GetExitCode() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.inner.State.ExitCode
}

func (d *Data) SetExitCode(exitCode int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.ExitCode = exitCode
}

func (d *Data) ClearState(s core.Step) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State = NodeState{}

	// Reset the state of the step
	d.inner.Step = s
}

func (d *Data) MarkError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.inner.State.Error = err
	d.inner.State.Status = status.NodeError
}

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Status int

const (
	StatusNone Status = iota
	StatusRunning
	StatusError
	StatusCancel
	StatusSuccess
	StatusQueued
	StatusPartialSuccess
)

func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusError:
		return "failed"
	case StatusCancel:
		return "canceled"
	case StatusSuccess:
		return "finished"
	case StatusQueued:
		return "queued"
	case StatusPartialSuccess:
		return "partial success"
	case StatusNone:
		fallthrough
	default:
		return "not started"
	}
}

// IsActive checks if the status is active.
func (s Status) IsActive() bool {
	return s == StatusRunning || s == StatusQueued
}

var (
	ErrUpstreamFailed  = fmt.Errorf("upstream failed")
	ErrUpstreamSkipped = fmt.Errorf("upstream skipped")
)

// Scheduler is a scheduler that runs a graph of steps.
type Scheduler struct {
	logDir        string
	maxActiveRuns int
	timeout       time.Duration
	delay         time.Duration
	dry           bool
	onExit        *digraph.Step
	onSuccess     *digraph.Step
	onFailure     *digraph.Step
	onCancel      *digraph.Step
	dagRunID      string

	canceled  int32
	mu        sync.RWMutex
	pause     time.Duration
	lastError error
	handlers  map[digraph.HandlerType]*Node

	metrics struct {
		startTime          time.Time
		totalNodes         int
		completedNodes     int
		failedNodes        int
		skippedNodes       int
		canceledNodes      int
		totalExecutionTime time.Duration
	}
}

func New(cfg *Config) *Scheduler {
	return &Scheduler{
		logDir:        cfg.LogDir,
		maxActiveRuns: cfg.MaxActiveSteps,
		timeout:       cfg.Timeout,
		delay:         cfg.Delay,
		dry:           cfg.Dry,
		onExit:        cfg.OnExit,
		onSuccess:     cfg.OnSuccess,
		onFailure:     cfg.OnFailure,
		onCancel:      cfg.OnCancel,
		dagRunID:      cfg.DAGRunID,
		pause:         time.Millisecond * 100,
	}
}

type Config struct {
	LogDir         string
	MaxActiveSteps int
	Timeout        time.Duration
	Delay          time.Duration
	Dry            bool
	OnExit         *digraph.Step
	OnSuccess      *digraph.Step
	OnFailure      *digraph.Step
	OnCancel       *digraph.Step
	DAGRunID       string
}

// Schedule runs the graph of steps.
func (sc *Scheduler) Schedule(ctx context.Context, graph *ExecutionGraph, progressCh chan *Node) error {
	if err := sc.setup(ctx); err != nil {
		return err
	}

	// Create a cancellable context for the entire execution
	var cancel context.CancelFunc
	if sc.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, sc.timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Start execution and ensure cleanup
	graph.Start()
	defer graph.Finish()

	// Initialize node count metrics
	sc.metrics.totalNodes = len(graph.nodes)

	// If one of the conditions does not met, cancel the execution.
	env := digraph.GetEnv(ctx)
	if err := EvalConditions(ctx, cmdutil.GetShellCommand(""), env.DAG.Preconditions); err != nil {
		logger.Info(ctx, "Preconditions are not met", "err", err)
		sc.Cancel(ctx, graph)
	}

	var wg = sync.WaitGroup{}

	for !graph.isFinished() {
		if sc.isCanceled() {
			break
		}

	NodesIteration:
		for _, node := range graph.nodes {
			if node.State().Status != NodeStatusNone || !isReady(ctx, graph, node) {
				continue NodesIteration
			}
			if sc.isCanceled() {
				break NodesIteration
			}
			if sc.maxActiveRuns > 0 && graph.runningCount() >= sc.maxActiveRuns {
				continue NodesIteration
			}

			wg.Add(1)

			logger.Info(ctx, "Step started", "step", node.Name())
			node.SetStatus(NodeStatusRunning)
			if progressCh != nil {
				progressCh <- node
			}

			go func(ctx context.Context, node *Node) {
				nodeCtx, nodeCancel := context.WithCancel(ctx)
				defer nodeCancel()

				// Recover from panics
				defer sc.recoverNodePanic(ctx, node)

				// Ensure node is finished and wg is decremented
				defer sc.finishNode(node, &wg)

				// Create span for step execution
				spanCtx := nodeCtx
				parentSpan := trace.SpanFromContext(nodeCtx)
				if parentSpan.SpanContext().IsValid() {
					spanAttrs := []attribute.KeyValue{
						attribute.String("step.name", node.Name()),
					}
					// Use the otel package to get the global tracer
					tracer := otel.Tracer("github.com/dagu-org/dagu")
					var span trace.Span
					spanCtx, span = tracer.Start(
						nodeCtx,
						fmt.Sprintf("Step: %s", node.Name()),
						trace.WithAttributes(spanAttrs...),
					)
					defer func() {
						// Set final step attributes
						nodeData := node.NodeData()
						span.SetAttributes(
							attribute.String("step.status", nodeData.State.Status.String()),
						)
						if nodeData.State.ExitCode != 0 {
							span.SetAttributes(attribute.Int("step.exit_code", nodeData.State.ExitCode))
						}
						span.End()
					}()
				}

				ctx = sc.setupEnviron(spanCtx, graph, node)

				// Check preconditions
				if !meetsPreconditions(ctx, node, progressCh) {
					return
				}

				setupSucceed := true
				if err := sc.setupNode(ctx, node); err != nil {
					setupSucceed = false
					sc.setLastError(err)
					node.MarkError(err)
				}

				ctx = node.SetupContextBeforeExec(ctx)

			ExecRepeat: // repeat execution
				for setupSucceed && !sc.isCanceled() {
					execErr := sc.execNode(ctx, node)
					isRetriable := sc.handleNodeExecutionError(ctx, graph, node, execErr)
					if isRetriable {
						continue ExecRepeat
					}

					if node.State().Status != NodeStatusCancel {
						node.IncDoneCount()
					}

					shouldRepeat := sc.shouldRepeatNode(ctx, node, execErr)
					if shouldRepeat && !sc.isCanceled() {
						sc.prepareNodeForRepeat(ctx, node, progressCh)
						continue
					}

					if execErr != nil && progressCh != nil {
						progressCh <- node
						return
					}

					break ExecRepeat
				}

				// If node is still in running state by now, it means it was not canceled
				// and it has completed its execution without errors.
				if node.State().Status == NodeStatusRunning {
					node.SetStatus(NodeStatusSuccess)
				}

				if err := sc.teardownNode(ctx, node); err != nil {
					sc.setLastError(err)
					node.SetStatus(NodeStatusError)
				}

				if progressCh != nil {
					progressCh <- node
				}
			}(ctx, node)

			time.Sleep(sc.delay) // TODO: check if this is necessary
		}

		time.Sleep(sc.pause) // avoid busy loop
	}

	wg.Wait()

	// Collect final metrics
	sc.metrics.totalExecutionTime = time.Since(sc.metrics.startTime)

	var eventHandlers []digraph.HandlerType
	switch sc.Status(ctx, graph) {
	case StatusSuccess:
		eventHandlers = append(eventHandlers, digraph.HandlerOnSuccess)

	case StatusPartialSuccess:
		// PartialSuccess is treated as success since primary work was completed
		// despite some non-critical failures that were allowed to continue
		eventHandlers = append(eventHandlers, digraph.HandlerOnSuccess)

	case StatusError:
		eventHandlers = append(eventHandlers, digraph.HandlerOnFailure)

	case StatusCancel:
		eventHandlers = append(eventHandlers, digraph.HandlerOnCancel)

	case StatusNone, StatusRunning, StatusQueued:
		// These states should not occur at this point
		logger.Warn(ctx, "Unexpected final status",
			"status", sc.Status(ctx, graph).String(),
			"dagRunId", sc.dagRunID)
	}

	eventHandlers = append(eventHandlers, digraph.HandlerOnExit)
	for _, handler := range eventHandlers {
		if handlerNode := sc.handlers[handler]; handlerNode != nil {
			logger.Info(ctx, "Handler execution started", "handler", handlerNode.Name())
			if err := sc.runEventHandler(ctx, graph, handlerNode); err != nil {
				sc.setLastError(err)
			}

			if progressCh != nil {
				progressCh <- handlerNode
			}
		}
	}

	logger.Debug(ctx, "Scheduler execution complete",
		"status", sc.Status(ctx, graph).String(),
		"lastError", sc.lastError,
	)

	return sc.lastError
}

func (sc *Scheduler) setLastError(err error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.lastError = err
}

func (sc *Scheduler) setupNode(ctx context.Context, node *Node) error {
	if !sc.dry {
		return node.Setup(ctx, sc.logDir, sc.dagRunID)
	}
	return nil
}

func (sc *Scheduler) teardownNode(ctx context.Context, node *Node) error {
	if !sc.dry {
		return node.Teardown(ctx)
	}
	return nil
}

func (sc *Scheduler) setupEnviron(ctx context.Context, graph *ExecutionGraph, node *Node) context.Context {
	env := executor.NewEnv(ctx, node.Step())

	// Populate step information for all nodes with IDs
	for _, n := range graph.nodes {
		if n.Step().ID != "" {
			stepInfo := cmdutil.StepInfo{
				Stdout:   n.GetStdout(),
				Stderr:   n.GetStderr(),
				ExitCode: strconv.Itoa(n.GetExitCode()),
			}
			env.StepMap[n.Step().ID] = stepInfo
		}
	}

	// Load output variables from predecessor nodes (dependencies)
	// This traverses backwards from the current node to find all nodes it depends on
	curr := node.id
	visited := make(map[int]struct{})
	queue := []int{}

	// Start with direct dependencies (nodes this node depends on)
	queue = append(queue, graph.To[curr]...)

	// Traverse all predecessor nodes
	for len(queue) > 0 {
		predID := queue[0]
		queue = queue[1:]

		if _, ok := visited[predID]; ok {
			continue
		}
		visited[predID] = struct{}{}

		// Add this node's dependencies to the queue
		queue = append(queue, graph.To[predID]...)

		// Load output variables from this predecessor node
		predNode := graph.nodeByID[predID]
		if predNode != nil && predNode.inner.State.OutputVariables != nil {
			env.LoadOutputVariables(predNode.inner.State.OutputVariables)
		}
	}

	return executor.WithEnv(ctx, env)
}

func (sc *Scheduler) setupEnvironEventHandler(ctx context.Context, graph *ExecutionGraph, node *Node) context.Context {
	env := executor.NewEnv(ctx, node.Step())

	// Populate step information for all nodes with IDs
	for _, n := range graph.nodes {
		if n.Step().ID != "" {
			stepInfo := cmdutil.StepInfo{
				Stdout:   n.GetStdout(),
				Stderr:   n.GetStderr(),
				ExitCode: strconv.Itoa(n.GetExitCode()),
			}
			env.StepMap[n.Step().ID] = stepInfo
		}
	}

	// get all output variables
	for _, node := range graph.nodes {
		if node.inner.State.OutputVariables == nil {
			continue
		}

		env.LoadOutputVariables(node.inner.State.OutputVariables)
	}

	return executor.WithEnv(ctx, env)
}

func (sc *Scheduler) execNode(ctx context.Context, node *Node) error {
	if !sc.dry {
		if err := node.Execute(ctx); err != nil {
			return fmt.Errorf("failed to execute step %q: %w", node.Name(), err)
		}
	}

	return nil
}

// Signal sends a signal to the scheduler.
// for a node with repeat policy, it does not stop the node and
// wait to finish current run.
func (sc *Scheduler) Signal(
	ctx context.Context, graph *ExecutionGraph, sig os.Signal, done chan bool, allowOverride bool,
) {
	if !sc.isCanceled() {
		sc.setCanceled()
	}

	for _, node := range graph.nodes {
		// for a repetitive task, we'll wait for the job to finish
		// until time reaches max wait time
		if !node.Step().RepeatPolicy.Repeat {
			node.Signal(ctx, sig, allowOverride)
		}
	}

	if done != nil {
		defer func() {
			done <- true
		}()

		for graph.IsRunning() {
			time.Sleep(sc.pause)
		}
	}
}

// Cancel sends -1 signal to all nodes.
func (sc *Scheduler) Cancel(ctx context.Context, g *ExecutionGraph) {
	sc.setCanceled()
	for _, node := range g.nodes {
		node.Cancel(ctx)
	}
}

// Status returns the status of the scheduler.
func (sc *Scheduler) Status(ctx context.Context, g *ExecutionGraph) Status {
	if sc.isCanceled() && !sc.isSucceed(g) {
		return StatusCancel
	}
	if !g.IsStarted() {
		return StatusNone
	}
	if g.IsRunning() {
		return StatusRunning
	}

	if sc.isPartialSuccess(ctx, g) {
		return StatusPartialSuccess
	}

	if sc.isError() {
		return StatusError
	}

	return StatusSuccess
}

func (sc *Scheduler) isError() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.lastError != nil
}

// HandlerNode returns the handler node with the given name.
func (sc *Scheduler) HandlerNode(name digraph.HandlerType) *Node {
	if v, ok := sc.handlers[name]; ok {
		return v
	}
	return nil
}

// isCanceled returns true if the scheduler is canceled.
func (sc *Scheduler) isCanceled() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.canceled == 1
}

func isReady(ctx context.Context, g *ExecutionGraph, node *Node) bool {
	ready := true
	for _, dep := range g.To[node.id] {
		dep := g.nodeByID[dep]

		switch dep.State().Status {
		case NodeStatusSuccess:
			continue

		case NodeStatusError:
			if dep.ShouldContinue(ctx) {
				continue
			}
			ready = false
			node.SetStatus(NodeStatusCancel)
			node.SetError(ErrUpstreamFailed)

		case NodeStatusSkipped:
			if dep.ShouldContinue(ctx) {
				continue
			}
			ready = false
			node.SetStatus(NodeStatusSkipped)
			node.SetError(ErrUpstreamSkipped)

		case NodeStatusCancel:
			ready = false
			node.SetStatus(NodeStatusCancel)

		case NodeStatusNone, NodeStatusRunning:
			ready = false

		default:
			ready = false

		}
	}
	return ready
}

func (sc *Scheduler) runEventHandler(ctx context.Context, graph *ExecutionGraph, node *Node) error {
	defer node.Finish()

	node.SetStatus(NodeStatusRunning)

	if !sc.dry {
		if err := node.Setup(ctx, sc.logDir, sc.dagRunID); err != nil {
			node.SetStatus(NodeStatusError)
			return nil
		}

		defer func() {
			_ = node.Teardown(ctx)
		}()

		ctx = sc.setupEnvironEventHandler(ctx, graph, node)
		if err := node.Execute(ctx); err != nil {
			node.SetStatus(NodeStatusError)
			return err
		}

		node.SetStatus(NodeStatusSuccess)
	} else {
		node.SetStatus(NodeStatusSuccess)
	}

	return nil
}

func (sc *Scheduler) setup(ctx context.Context) (err error) {
	// Apply environment variables specified for the DAG.
	env := digraph.GetEnv(ctx)
	env.ApplyEnvs(ctx)

	if !sc.dry {
		if err = os.MkdirAll(sc.logDir, 0750); err != nil {
			err = fmt.Errorf("failed to create log directory: %w", err)
			return err
		}
	}

	// Initialize handlers
	sc.handlers = map[digraph.HandlerType]*Node{}
	if sc.onExit != nil {
		sc.handlers[digraph.HandlerOnExit] = &Node{Data: newSafeData(NodeData{Step: *sc.onExit})}
	}
	if sc.onSuccess != nil {
		sc.handlers[digraph.HandlerOnSuccess] = &Node{Data: newSafeData(NodeData{Step: *sc.onSuccess})}
	}
	if sc.onFailure != nil {
		sc.handlers[digraph.HandlerOnFailure] = &Node{Data: newSafeData(NodeData{Step: *sc.onFailure})}
	}
	if sc.onCancel != nil {
		sc.handlers[digraph.HandlerOnCancel] = &Node{Data: newSafeData(NodeData{Step: *sc.onCancel})}
	}

	// Initialize metrics
	sc.metrics.startTime = time.Now()

	// Log scheduler setup
	logger.Debug(ctx, "Scheduler setup complete",
		"dagRunId", sc.dagRunID,
		"maxActiveRuns", sc.maxActiveRuns,
		"timeout", sc.timeout,
		"dry", sc.dry)

	return err
}

func (sc *Scheduler) setCanceled() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.canceled = 1
}

func (sc *Scheduler) isSucceed(g *ExecutionGraph) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	for _, node := range g.nodes {
		nodeStatus := node.State().Status
		if nodeStatus == NodeStatusSuccess || nodeStatus == NodeStatusSkipped {
			continue
		}
		return false
	}
	return true
}

// isPartialSuccess checks if the DAG completed with some failures that were allowed to continue.
// This represents scenarios where execution continued despite failures due to continueOn conditions.
func (sc *Scheduler) isPartialSuccess(ctx context.Context, g *ExecutionGraph) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	hasFailuresWithContinueOn := false
	hasSuccessfulNodes := false

	// First pass: check if any failed node is NOT allowed to continue
	// If so, this is an error, not partial success
	for _, node := range g.nodes {
		if node.State().Status == NodeStatusError {
			if !node.ShouldContinue(ctx) {
				// Found a failed node that was NOT allowed to continue
				// This disqualifies the DAG from being partial success
				return false
			}
		}
	}

	// Second pass: check for partial success conditions
	for _, node := range g.nodes {
		switch node.State().Status {
		case NodeStatusSuccess:
			hasSuccessfulNodes = true
		case NodeStatusError:
			if node.ShouldContinue(ctx) && !node.ShouldMarkSuccess(ctx) {
				hasFailuresWithContinueOn = true
			}
		case NodeStatusNone, NodeStatusRunning, NodeStatusCancel, NodeStatusSkipped:
			// These statuses don't affect partial success determination, but are needed for linter
		}
	}

	// Partial success requires:
	// 1. At least one failed node with continueOn (some non-critical failures)
	// 2. No failed nodes without continueOn (checked in first pass)
	// Note: Skipped nodes alone do not count as successful completion
	return hasSuccessfulNodes && hasFailuresWithContinueOn
}

func (sc *Scheduler) isTimeout(startedAt time.Time) bool {
	return sc.timeout > 0 && time.Since(startedAt) > sc.timeout
}

// GetMetrics returns the current metrics for the scheduler
func (sc *Scheduler) GetMetrics() map[string]any {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	metrics := map[string]any{
		"totalNodes":         sc.metrics.totalNodes,
		"completedNodes":     sc.metrics.completedNodes,
		"failedNodes":        sc.metrics.failedNodes,
		"skippedNodes":       sc.metrics.skippedNodes,
		"canceledNodes":      sc.metrics.canceledNodes,
		"totalExecutionTime": sc.metrics.totalExecutionTime.String(),
	}

	return metrics
}

// shouldRetryNode handles the retry logic for a node based on exit codes and retry policy
func (sc *Scheduler) shouldRetryNode(ctx context.Context, node *Node, execErr error) (shouldRetry bool) {
	var exitCode int
	var exitCodeFound bool

	// Try to extract the exec.ExitError using errors.As
	var exitErr *exec.ExitError
	if errors.As(execErr, &exitErr) {
		exitCode = exitErr.ExitCode()
		exitCodeFound = true
		logger.Debug(ctx, "Found exit error", "error", execErr, "exitCode", exitCode)
	}

	if !exitCodeFound {
		// Try to parse exit code from error string
		errStr := execErr.Error()
		if code, found := parseExitCodeFromError(errStr); found {
			exitCode = code
			exitCodeFound = true
			logger.Debug(ctx, "Parsed exit code from error string", "error", errStr, "exitCode", exitCode)
		} else if strings.Contains(errStr, "signal:") {
			// Handle signal termination
			exitCode = -1
			exitCodeFound = true
			logger.Debug(ctx, "Process terminated by signal", "error", errStr)
		}
	}

	if !exitCodeFound {
		logger.Debug(ctx, "Could not determine exit code", "error", execErr, "errorType", fmt.Sprintf("%T", execErr))
		// Default to exit code 1 if we can't determine the actual code
		exitCode = 1
	}

	shouldRetry = node.retryPolicy.ShouldRetry(exitCode)
	logger.Debug(ctx, "Checking retry policy", "exitCode", exitCode, "allowedCodes", node.retryPolicy.ExitCodes, "shouldRetry", shouldRetry)

	if !shouldRetry {
		// finish the node with error
		node.SetStatus(NodeStatusError)
		node.MarkError(execErr)
		sc.setLastError(execErr)
		return false
	}

	logger.Info(ctx, "Step execution failed. Retrying...", "step", node.Name(), "error", execErr, "retry", node.GetRetryCount(), "exitCode", exitCode)

	// Set the node status to none so that it can be retried
	node.IncRetryCount()
	time.Sleep(node.retryPolicy.Interval)
	node.SetRetriedAt(time.Now())
	node.SetStatus(NodeStatusRunning)
	return true
}

// recoverNodePanic handles panic recovery for a node goroutine.
func (sc *Scheduler) recoverNodePanic(ctx context.Context, node *Node) {
	if panicObj := recover(); panicObj != nil {
		stack := string(debug.Stack())
		err := fmt.Errorf("panic recovered in node %s: %v\n%s", node.Name(), panicObj, stack)
		logger.Error(ctx, "Panic occurred",
			"error", err,
			"step", node.Name(),
			"stack", stack,
			"dagRunId", sc.dagRunID)
		node.MarkError(err)
		sc.setLastError(err)

		// Update metrics for failed node
		sc.mu.Lock()
		sc.metrics.failedNodes++
		sc.mu.Unlock()
	}
}

// finishNode updates metrics and finalizes the node.
func (sc *Scheduler) finishNode(node *Node, wg *sync.WaitGroup) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	switch node.State().Status {
	case NodeStatusSuccess:
		sc.metrics.completedNodes++
	case NodeStatusError:
		sc.metrics.failedNodes++
	case NodeStatusSkipped:
		sc.metrics.skippedNodes++
	case NodeStatusCancel:
		sc.metrics.canceledNodes++
	case NodeStatusNone, NodeStatusRunning:
		// Should not happen at this point
	}

	node.Finish()
	wg.Done()
}

// checkPreconditions evaluates the preconditions for a node and updates its status accordingly.
func meetsPreconditions(ctx context.Context, node *Node, progressCh chan *Node) bool {
	err := node.evalPreconditions(ctx)
	if err != nil {
		// Precondition not met, skip the node
		node.SetStatus(NodeStatusSkipped)
		if !errors.Is(err, ErrConditionNotMet) {
			node.SetError(err)
		}
		if progressCh != nil {
			progressCh <- node
		}
		return false
	}
	return true
}

// handleNodeExecutionError processes execution errors for a node and determines if execution should be retried.
// Returns true if the execution should be retried, false otherwise.
func (sc *Scheduler) handleNodeExecutionError(ctx context.Context, graph *ExecutionGraph, node *Node, execErr error) (isRetriable bool) {
	if execErr == nil {
		return false // no error, nothing to handle
	}

	status := node.State().Status
	switch {
	case status == NodeStatusSuccess || status == NodeStatusCancel:
		// do nothing

	case sc.isTimeout(graph.startedAt):
		logger.Info(ctx, "Step deadline exceeded", "step", node.Name(), "error", execErr)
		node.SetStatus(NodeStatusCancel)
		sc.setLastError(execErr)

	case sc.isCanceled():
		sc.setLastError(execErr)

	case node.retryPolicy.Limit > node.GetRetryCount():
		if sc.shouldRetryNode(ctx, node, execErr) {
			return true
		}

	default:
		// node execution error is unexpected and unrecoverable
		node.SetStatus(NodeStatusError)
		if node.ShouldMarkSuccess(ctx) {
			// mark as success if the node should be force marked as success
			// i.e. continueOn.markSuccess is set to true
			node.SetStatus(NodeStatusSuccess)
		} else {
			node.MarkError(execErr)
			sc.setLastError(execErr)
		}
	}

	return false
}

// shouldRepeatNode determines if a node should be repeated based on its repeat policy
func (sc *Scheduler) shouldRepeatNode(ctx context.Context, node *Node, execErr error) bool {
	step := node.Step()

	// Check if repeat limit has been reached
	if step.RepeatPolicy.Limit > 0 && node.State().DoneCount >= step.RepeatPolicy.Limit {
		return false
	}

	if step.RepeatPolicy.Condition != nil {
		return sc.evaluateRepeatCondition(ctx, node, &step)
	}

	if len(step.RepeatPolicy.ExitCode) > 0 {
		// Repeat if last exit code matches any in ExitCode
		lastExit := node.State().ExitCode
		return slices.Contains(step.RepeatPolicy.ExitCode, lastExit)
	}

	if step.RepeatPolicy.Repeat {
		// Unconditional repeat
		return execErr == nil || step.ContinueOn.Failure
	}

	return false
}

// evaluateRepeatCondition evaluates the condition-based repeat policy
func (sc *Scheduler) evaluateRepeatCondition(ctx context.Context, node *Node, step *digraph.Step) bool {
	// Ensure node's own output variables are reloaded before evaluating the condition
	if node.inner.State.OutputVariables != nil {
		env := executor.GetEnv(ctx)
		env.ForceLoadOutputVariables(node.inner.State.OutputVariables)
		ctx = executor.WithEnv(ctx, env)
	}

	shell := cmdutil.GetShellCommand(step.Shell)
	err := EvalCondition(ctx, shell, step.RepeatPolicy.Condition)

	if step.RepeatPolicy.Condition.Expected != "" {
		// Repeat as long as condition does NOT match expected (err != nil)
		return err != nil
	}

	// Repeat as long as it returns exit code 0 (err == nil)
	return err == nil
}

// prepareNodeForRepeat sets up a node for repetition
func (sc *Scheduler) prepareNodeForRepeat(ctx context.Context, node *Node, progressCh chan *Node) {
	step := node.Step()

	node.SetStatus(NodeStatusRunning) // reset status to running for the repeat
	if sc.lastError == node.Error() {
		sc.setLastError(nil) // clear last error if we are repeating
	}
	logger.Info(ctx, "Step will be repeated", "step", node.Name(), "interval", step.RepeatPolicy.Interval)
	time.Sleep(step.RepeatPolicy.Interval)
	node.SetRepeated(true) // mark as repeated
	logger.Info(ctx, "Repeating step", "step", node.Name())

	if progressCh != nil {
		progressCh <- node
	}
}

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/signal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

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

	handlerMu sync.RWMutex
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
			if node.State().Status != status.NodeNone || !isReady(ctx, graph, node) {
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

			if err := sc.setupNode(ctx, node); err != nil {
				sc.setLastError(err)
				node.MarkError(err)
				node.SetStatus(status.NodeError)
				sc.finishNode(node, &wg)
				continue NodesIteration
			}

			node.SetStatus(status.NodeRunning)
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

				ctx = node.SetupContextBeforeExec(ctx)

			ExecRepeat: // repeat execution
				for !sc.isCanceled() {
					execErr := sc.execNode(ctx, node)
					isRetriable := sc.handleNodeExecutionError(ctx, graph, node, execErr)
					if isRetriable {
						continue ExecRepeat
					}

					if node.State().Status != status.NodeCancel {
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
				if node.State().Status == status.NodeRunning {
					node.SetStatus(status.NodeSuccess)
				}

				if err := sc.teardownNode(ctx, node); err != nil {
					sc.setLastError(err)
					node.SetStatus(status.NodeError)
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
	case status.Success:
		eventHandlers = append(eventHandlers, digraph.HandlerOnSuccess)

	case status.PartialSuccess:
		// PartialSuccess is treated as success since primary work was completed
		// despite some non-critical failures that were allowed to continue
		eventHandlers = append(eventHandlers, digraph.HandlerOnSuccess)

	case status.Error:
		eventHandlers = append(eventHandlers, digraph.HandlerOnFailure)

	case status.Cancel:
		eventHandlers = append(eventHandlers, digraph.HandlerOnCancel)

	case status.None, status.Running, status.Queued:
		// These states should not occur at this point
		logger.Warn(ctx, "Unexpected final status",
			"status", sc.Status(ctx, graph).String(),
			"dagRunId", sc.dagRunID)
	}

	eventHandlers = append(eventHandlers, digraph.HandlerOnExit)

	sc.handlerMu.RLock()
	defer sc.handlerMu.RUnlock()

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

	// Add step-level environment variables
	envVars := &digraph.SyncMap{}
	for _, v := range node.Step().Env {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			logger.Error(ctx, "Invalid environment variable format", "var", v)
			continue
		}
		val, err := env.EvalString(ctx, v)
		if err != nil {
			logger.Error(ctx, "Failed to evaluate environment variable", "var", v, "err", err)
			continue
		}
		envVars.Store(parts[0], val)
	}

	env.ForceLoadOutputVariables(envVars)

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
	isTermination := signal.IsTerminationSignalOS(sig)
	if !sc.isCanceled() && isTermination {
		sc.setCanceled()
	}

	for _, node := range graph.nodes {
		// for a repetitive task, we'll wait for the job to finish
		// until time reaches max wait time
		if node.Step().RepeatPolicy.RepeatMode != "" {
			logger.Info(ctx, "Waiting the repeat node finish", "step", node.Step().Name)
			continue
		}
		node.Signal(ctx, sig, allowOverride)
	}

	if done != nil && isTermination {
		defer func() {
			for graph.IsRunning() {
				time.Sleep(sc.pause)
			}
			done <- true
		}()
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
func (sc *Scheduler) Status(ctx context.Context, g *ExecutionGraph) status.Status {
	if sc.isCanceled() && !sc.isSucceed(g) {
		return status.Cancel
	}
	if !g.IsStarted() {
		return status.None
	}
	if g.IsRunning() {
		return status.Running
	}

	if sc.isPartialSuccess(ctx, g) {
		return status.PartialSuccess
	}

	if sc.isError() {
		return status.Error
	}

	return status.Success
}

func (sc *Scheduler) isError() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.lastError != nil
}

// HandlerNode returns the handler node with the given name.
func (sc *Scheduler) HandlerNode(name digraph.HandlerType) *Node {
	sc.handlerMu.RLock()
	defer sc.handlerMu.RUnlock()
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
		case status.NodeSuccess:
			continue

		case status.NodePartialSuccess:
			// Partial success is treated like success for dependencies
			continue

		case status.NodeError:
			if dep.ShouldContinue(ctx) {
				continue
			}
			ready = false
			node.SetStatus(status.NodeCancel)
			node.SetError(ErrUpstreamFailed)

		case status.NodeSkipped:
			if dep.ShouldContinue(ctx) {
				continue
			}
			ready = false
			node.SetStatus(status.NodeSkipped)
			node.SetError(ErrUpstreamSkipped)

		case status.NodeCancel:
			ready = false
			node.SetStatus(status.NodeCancel)

		case status.NodeNone, status.NodeRunning:
			ready = false

		default:
			ready = false

		}
	}
	return ready
}

func (sc *Scheduler) runEventHandler(ctx context.Context, graph *ExecutionGraph, node *Node) error {
	defer node.Finish()

	if !sc.dry {
		if err := node.Setup(ctx, sc.logDir, sc.dagRunID); err != nil {
			node.SetStatus(status.NodeError)
			return nil
		}

		defer func() {
			_ = node.Teardown(ctx)
		}()

		node.SetStatus(status.NodeRunning)

		ctx = sc.setupEnvironEventHandler(ctx, graph, node)
		if err := node.Execute(ctx); err != nil {
			node.SetStatus(status.NodeError)
			return err
		}

		node.SetStatus(status.NodeSuccess)
	} else {
		node.SetStatus(status.NodeSuccess)
	}

	return nil
}

func (sc *Scheduler) setup(ctx context.Context) (err error) {
	if !sc.dry {
		if err = os.MkdirAll(sc.logDir, 0750); err != nil {
			err = fmt.Errorf("failed to create log directory: %w", err)
			return err
		}
	}

	// Initialize handlers
	sc.handlerMu.Lock()
	defer sc.handlerMu.Unlock()
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
		if nodeStatus == status.NodeSuccess || nodeStatus == status.NodeSkipped || nodeStatus == status.NodePartialSuccess {
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
		if node.State().Status == status.NodeError {
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
		case status.NodeSuccess:
			hasSuccessfulNodes = true
		case status.NodeError:
			if node.ShouldContinue(ctx) && !node.ShouldMarkSuccess(ctx) {
				hasFailuresWithContinueOn = true
			}
		case status.NodePartialSuccess:
			// Partial success at node level contributes to overall partial success
			hasFailuresWithContinueOn = true
			hasSuccessfulNodes = true
		case status.NodeNone, status.NodeRunning, status.NodeCancel, status.NodeSkipped:
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
		logger.Debug(ctx, "Found exit error", "err", execErr, "exitCode", exitCode)
	}

	if !exitCodeFound {
		// Try to parse exit code from error string
		errStr := execErr.Error()
		if code, found := parseExitCodeFromError(errStr); found {
			exitCode = code
			exitCodeFound = true
			logger.Debug(ctx, "Parsed exit code from error string", "err", errStr, "exitCode", exitCode)
		} else if strings.Contains(errStr, "signal:") {
			// Handle signal termination
			exitCode = -1
			exitCodeFound = true
			logger.Debug(ctx, "Process terminated by signal", "err", errStr)
		}
	}

	if !exitCodeFound {
		logger.Debug(ctx, "Could not determine exit code", "err", execErr, "errorType", fmt.Sprintf("%T", execErr))
		// Default to exit code 1 if we can't determine the actual code
		exitCode = 1
	}

	shouldRetry = node.retryPolicy.ShouldRetry(exitCode)
	logger.Debug(ctx, "Checking retry policy", "exitCode", exitCode, "allowedCodes", node.retryPolicy.ExitCodes, "shouldRetry", shouldRetry)

	if !shouldRetry {
		// finish the node with error
		node.SetStatus(status.NodeError)
		node.MarkError(execErr)
		sc.setLastError(execErr)
		return false
	}

	logger.Info(ctx, "Step execution failed. Retrying...", "step", node.Name(), "err", execErr, "retry", node.GetRetryCount(), "exitCode", exitCode)

	// Set the node status to none so that it can be retried
	node.IncRetryCount()
	interval := calculateBackoffInterval(
		node.Step().RetryPolicy.Interval,
		node.Step().RetryPolicy.Backoff,
		node.Step().RetryPolicy.MaxInterval,
		node.GetRetryCount()-1, // -1 because we just incremented
	)
	time.Sleep(interval)
	node.SetRetriedAt(time.Now())
	node.SetStatus(status.NodeRunning)
	return true
}

// recoverNodePanic handles panic recovery for a node goroutine.
func (sc *Scheduler) recoverNodePanic(ctx context.Context, node *Node) {
	if panicObj := recover(); panicObj != nil {
		stack := string(debug.Stack())
		err := fmt.Errorf("panic recovered in node %s: %v\n%s", node.Name(), panicObj, stack)
		logger.Error(ctx, "Panic occurred",
			"err", err,
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
	case status.NodeSuccess:
		sc.metrics.completedNodes++
	case status.NodeError:
		sc.metrics.failedNodes++
	case status.NodeSkipped:
		sc.metrics.skippedNodes++
	case status.NodeCancel:
		sc.metrics.canceledNodes++
	case status.NodePartialSuccess:
		sc.metrics.completedNodes++ // Count partial success as completed
	case status.NodeNone, status.NodeRunning:
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
		node.SetStatus(status.NodeSkipped)
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

	s := node.State().Status
	switch {
	case s == status.NodeSuccess || s == status.NodeCancel || s == status.NodePartialSuccess:
		// do nothing

	case sc.isTimeout(graph.startedAt):
		logger.Info(ctx, "Step deadline exceeded", "step", node.Name(), "error", execErr)
		node.SetStatus(status.NodeCancel)
		sc.setLastError(execErr)

	case sc.isCanceled():
		sc.setLastError(execErr)

	case node.retryPolicy.Limit > node.GetRetryCount():
		if sc.shouldRetryNode(ctx, node, execErr) {
			return true
		}

	default:
		// node execution error is unexpected and unrecoverable
		node.SetStatus(status.NodeError)
		if node.ShouldMarkSuccess(ctx) {
			// mark as success if the node should be force marked as success
			// i.e. continueOn.markSuccess is set to true
			node.SetStatus(status.NodeSuccess)
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
	rp := step.RepeatPolicy

	// First, check the hard limit. This overrides everything.
	if rp.Limit > 0 && node.State().DoneCount >= rp.Limit {
		return false
	}

	switch rp.RepeatMode {
	case digraph.RepeatModeWhile:
		// It's a 'while' loop. Repeat while a condition is true.
		if rp.Condition != nil {
			// Ensure node's own output variables are reloaded before evaluating the condition.
			if node.inner.State.OutputVariables != nil {
				env := executor.GetEnv(ctx)
				env.ForceLoadOutputVariables(node.inner.State.OutputVariables)
				ctx = executor.WithEnv(ctx, env)
			}
			shell := cmdutil.GetShellCommand(node.Step().Shell)
			err := EvalCondition(ctx, shell, rp.Condition)
			return (err == nil) // Repeat if condition is met (no error)
		} else if len(rp.ExitCode) > 0 {
			lastExit := node.State().ExitCode
			return slices.Contains(rp.ExitCode, lastExit) // Repeat if exit code matches
		} else {
			// No specific condition, so it's an unconditional 'while'.
			// Repeat as long as the step itself succeeds.
			return (execErr == nil)
		}
	case digraph.RepeatModeUntil:
		// It's an 'until' loop. Repeat until a condition is true.
		if rp.Condition != nil {
			// Ensure node's own output variables are reloaded before evaluating the condition.
			if node.inner.State.OutputVariables != nil {
				env := executor.GetEnv(ctx)
				env.ForceLoadOutputVariables(node.inner.State.OutputVariables)
				ctx = executor.WithEnv(ctx, env)
			}
			shell := cmdutil.GetShellCommand(node.Step().Shell)
			err := EvalCondition(ctx, shell, rp.Condition)
			return (err != nil) // Repeat if condition is NOT met (error)
		} else if len(rp.ExitCode) > 0 {
			lastExit := node.State().ExitCode
			return !slices.Contains(rp.ExitCode, lastExit) // Repeat if exit code does NOT match
		} else {
			// No specific condition, so it's an unconditional 'until'.
			// Repeat until the step itself succeeds (i.e., repeat on failure).
			return (execErr != nil)
		}
	}

	return false
}

// prepareNodeForRepeat sets up a node for repetition
func (sc *Scheduler) prepareNodeForRepeat(ctx context.Context, node *Node, progressCh chan *Node) {
	step := node.Step()

	node.SetStatus(status.NodeRunning) // reset status to running for the repeat
	if sc.lastError == node.Error() {
		sc.setLastError(nil) // clear last error if we are repeating
	}
	logger.Info(ctx, "Step will be repeated", "step", node.Name(), "interval", step.RepeatPolicy.Interval)
	interval := calculateBackoffInterval(
		step.RepeatPolicy.Interval,
		step.RepeatPolicy.Backoff,
		step.RepeatPolicy.MaxInterval,
		node.State().DoneCount,
	)
	time.Sleep(interval)
	node.SetRepeated(true) // mark as repeated
	logger.Info(ctx, "Repeating step", "step", node.Name())

	if progressCh != nil {
		progressCh <- node
	}
}

func calculateBackoffInterval(interval time.Duration, backoff float64, maxInterval time.Duration, attemptCount int) time.Duration {
	if backoff > 0 {
		sleeptime := float64(interval) * math.Pow(backoff, float64(attemptCount))
		if maxInterval > 0 && time.Duration(sleeptime) > maxInterval {
			return maxInterval
		}
		return time.Duration(sleeptime)
	}
	return interval
}

package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/signal"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrUpstreamFailed   = fmt.Errorf("upstream failed")
	ErrUpstreamSkipped  = fmt.Errorf("upstream skipped")
	ErrDeadlockDetected = errors.New("deadlock detected: no runnable nodes but DAG not finished")
)

// Runner runs a plan of steps.
type Runner struct {
	logDir        string
	maxActiveRuns int
	timeout       time.Duration
	delay         time.Duration
	dry           bool
	onExit        *core.Step
	onSuccess     *core.Step
	onFailure     *core.Step
	onCancel      *core.Step
	dagRunID      string

	canceled  int32
	mu        sync.RWMutex
	pause     time.Duration
	lastError error

	handlerMu sync.RWMutex
	handlers  map[core.HandlerType]*Node

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

func New(cfg *Config) *Runner {
	return &Runner{
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
	OnExit         *core.Step
	OnSuccess      *core.Step
	OnFailure      *core.Step
	OnCancel       *core.Step
	DAGRunID       string
}

// Run runs the plan of steps.
func (r *Runner) Run(ctx context.Context, plan *Plan, progressCh chan *Node) error {
	if err := r.setup(ctx); err != nil {
		return err
	}

	// Create a cancellable context for the entire execution
	var cancel context.CancelFunc
	if r.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	defer plan.Finish()

	// Initialize node count metrics
	nodes := plan.Nodes()
	r.metrics.totalNodes = len(nodes)

	// If one of the conditions does not met, cancel the execution.
	rCtx := GetDAGContext(ctx)
	var shell []string
	if rCtx.DAG.Shell != "" {
		shell = append([]string{rCtx.DAG.Shell}, rCtx.DAG.ShellArgs...)
	}
	if err := EvalConditions(ctx, shell, rCtx.DAG.Preconditions); err != nil {
		logger.Info(ctx, "Preconditions are not met", tag.Error(err))
		r.Cancel(plan)
	}

	// Channels for event loop
	// Buffer size = total nodes to avoid blocking
	readyCh := make(chan *Node, len(nodes))
	doneCh := make(chan *Node, len(nodes))

	// Find initial ready nodes
	for _, node := range nodes {
		if node.State().Status == core.NodeNotStarted && isReady(ctx, plan, node) {
			logger.Debug(ctx, "Initial node ready", tag.Step(node.Name()))
			readyCh <- node
		}
	}

	var wg sync.WaitGroup
	running := 0

	// Event loop
	ctxDoneCh := ctx.Done()
	for !plan.CheckFinished() {
		// If canceled and no running nodes, we are done
		if r.isCanceled() && running == 0 {
			break
		}

		var activeReadyCh chan *Node
		// Only accept new nodes if:
		// 1. Not canceled
		// 2. maxActiveRuns is 0 (unlimited) OR running < maxActiveRuns
		if !r.isCanceled() && (r.maxActiveRuns == 0 || running < r.maxActiveRuns) {
			activeReadyCh = readyCh
		}

		// Deadlock detection: if no nodes are running, no nodes are ready, and the graph is not finished,
		// then we are stuck (nodes are waiting for dependencies that will never be satisfied).
		if running == 0 && len(activeReadyCh) == 0 && !plan.CheckFinished() {
			r.setLastError(ErrDeadlockDetected)
			logger.Error(ctx, "Deadlock detected: no runnable nodes remaining")
			break
		}

		select {
		case node := <-activeReadyCh:
			logger.Debug(ctx, "Processing ready node", tag.Step(node.Name()))
			// Double check status
			if node.State().Status != core.NodeNotStarted {
				continue
			}

			running++
			wg.Add(1)

			logger.Info(ctx, "Step started", tag.Step(node.Name()))

			go func(n *Node) {
				// Set step context for all logs in this goroutine
				ctx := logger.WithValues(ctx, tag.Step(n.Name()))

				// Ensure node is finished and wg is decremented
				defer r.finishNode(n, &wg)
				// Recover from panics
				defer r.recoverNodePanic(ctx, n)
				// Signal completion to runner loop
				defer func() {
					doneCh <- n
				}()

				if err := r.prepareNode(ctx, n); err != nil {
					r.setLastError(err)
					n.MarkError(err)
					n.SetStatus(core.NodeFailed)
					return
				}

				n.SetStatus(core.NodeRunning)
				if progressCh != nil {
					progressCh <- n
				}

				r.runNodeExecution(ctx, plan, n, progressCh)
			}(node)

			if r.delay > 0 {
				time.Sleep(r.delay)
			}

		case node := <-doneCh:
			logger.Debug(ctx, "Node execution finished", tag.Step(node.Name()))
			running--
			r.processCompletedNode(ctx, plan, node, readyCh)

		case <-ctxDoneCh:
			r.mu.Lock()
			if r.lastError == nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				r.lastError = ctx.Err()
			}
			r.mu.Unlock()
			ctxDoneCh = nil
		}
	}

	wg.Wait()

	// Collect final metrics
	r.metrics.totalExecutionTime = time.Since(r.metrics.startTime)

	var eventHandlers []core.HandlerType
	switch r.Status(ctx, plan) {
	case core.Succeeded:
		eventHandlers = append(eventHandlers, core.HandlerOnSuccess)

	case core.PartiallySucceeded:
		// PartialSuccess is treated as success since primary work was completed
		// despite some non-critical failures that were allowed to continue
		eventHandlers = append(eventHandlers, core.HandlerOnSuccess)

	case core.Failed:
		eventHandlers = append(eventHandlers, core.HandlerOnFailure)

	case core.Aborted:
		eventHandlers = append(eventHandlers, core.HandlerOnCancel)

	case core.NotStarted, core.Running, core.Queued:
		// These states should not occur at this point
		logger.Warn(ctx, "Unexpected final status",
			tag.Status(r.Status(ctx, plan).String()),
		)
	}

	eventHandlers = append(eventHandlers, core.HandlerOnExit)

	r.handlerMu.RLock()
	defer r.handlerMu.RUnlock()

	for _, handler := range eventHandlers {
		if handlerNode := r.handlers[handler]; handlerNode != nil {
			logger.Debug(ctx, "Handler execution started",
				tag.Handler(handlerNode.Name()),
			)
			if err := r.runEventHandler(ctx, plan, handlerNode); err != nil {
				r.setLastError(err)
			}

			if progressCh != nil {
				progressCh <- handlerNode
			}
		}
	}

	logger.Debug(ctx, "Runner execution complete",
		tag.Status(r.Status(ctx, plan).String()),
		tag.Error(r.lastError),
	)

	return r.lastError
}

func (r *Runner) processCompletedNode(ctx context.Context, plan *Plan, node *Node, readyCh chan *Node) {
	if r.isCanceled() {
		return
	}

	// Queue of nodes to process (nodes that just finished)
	queue := []*Node{node}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, childID := range plan.Dependents(curr.id) {
			child := plan.GetNode(childID)
			if child.State().Status == core.NodeNotStarted {
				if isReady(ctx, plan, child) {
					logger.Debug(ctx, "Dependency satisfied, node ready",
						tag.Step(child.Name()),
						tag.Parent(curr.Name()),
					)
					readyCh <- child
				} else if child.State().Status != core.NodeNotStarted {
					// Child was marked as Aborted/Skipped/Failed by isReady
					// Add to queue to propagate to its children
					queue = append(queue, child)
				}
			}
		}
	}
}

func (r *Runner) runNodeExecution(ctx context.Context, plan *Plan, node *Node, progressCh chan *Node) {
	logger.Debug(ctx, "Starting node execution")
	nodeCtx, nodeCancel := context.WithCancel(ctx)
	defer nodeCancel()

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

	ctx = r.setupVariables(spanCtx, plan, node)

	// Check preconditions
	logger.Debug(ctx, "Checking preconditions")
	if !meetsPreconditions(ctx, node, progressCh) {
		return
	}

	ctx = node.SetupEnv(ctx)

ExecRepeat: // repeat execution
	for !r.isCanceled() {
		logger.Debug(ctx, "Executing node loop")
		execErr := r.execNode(ctx, node)
		isRetriable := r.handleNodeExecutionError(ctx, plan, node, execErr)
		if isRetriable {
			continue ExecRepeat
		}

		if node.State().Status != core.NodeAborted {
			node.IncDoneCount()
		}

		shouldRepeat := r.shouldRepeatNode(ctx, node, execErr)
		if shouldRepeat && !r.isCanceled() {
			r.prepareNodeForRepeat(ctx, node, progressCh)
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
	if node.State().Status == core.NodeRunning {
		node.SetStatus(core.NodeSucceeded)
	}

	if err := r.teardownNode(node); err != nil {
		r.setLastError(err)
		node.SetStatus(core.NodeFailed)
	}

	if progressCh != nil {
		progressCh <- node
	}
}

func (r *Runner) setLastError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastError = err
}

func (r *Runner) prepareNode(ctx context.Context, node *Node) error {
	if !r.dry {
		return node.Prepare(ctx, r.logDir, r.dagRunID)
	}
	return nil
}

func (r *Runner) teardownNode(node *Node) error {
	if !r.dry {
		return node.Teardown()
	}
	return nil
}

func (r *Runner) setupVariables(ctx context.Context, plan *Plan, node *Node) context.Context {
	env := NewPlanEnv(ctx, node.Step(), plan)

	// Load output variables from predecessor nodes (dependencies)
	// This traverses backwards from the current node to find all nodes it depends on
	curr := node.id
	visited := make(map[int]struct{})
	queue := []int{}

	// Start with direct dependencies (nodes this node depends on)
	queue = append(queue, plan.Dependencies(curr)...)

	// Traverse all predecessor nodes
	for len(queue) > 0 {
		predID := queue[0]
		queue = queue[1:]

		if _, ok := visited[predID]; ok {
			continue
		}
		visited[predID] = struct{}{}

		// Add this node's dependencies to the queue
		queue = append(queue, plan.Dependencies(predID)...)

		// Load output variables from this predecessor node
		predNode := plan.GetNode(predID)
		if predNode != nil && predNode.inner.State.OutputVariables != nil {
			env.LoadOutputVariables(predNode.inner.State.OutputVariables)
		}
	}

	// Add step-level environment variables
	envVars := &collections.SyncMap{}
	for _, v := range node.Step().Env {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			logger.Error(ctx, "Invalid environment variable format",
				slog.String("var", v),
			)
			continue
		}
		// Evaluate only the value part (parts[1]), not the entire "KEY=value" string
		evaluatedValue, err := env.EvalString(ctx, parts[1])
		if err != nil {
			logger.Error(ctx, "Failed to evaluate environment variable",
				slog.String("var", v),
				tag.Error(err),
			)
			continue
		}
		// Store as "KEY=evaluatedValue" format
		envVars.Store(parts[0], parts[0]+"="+evaluatedValue)
	}

	env.ForceLoadOutputVariables(envVars)

	return WithEnv(ctx, env)
}

func (r *Runner) setupEnvironEventHandler(ctx context.Context, plan *Plan, node *Node) context.Context {
	env := NewPlanEnv(ctx, node.Step(), plan)
	env.Envs[execution.EnvKeyDAGRunStatus] = r.Status(ctx, plan).String()

	// get all output variables
	for _, node := range plan.Nodes() {
		if node.inner.State.OutputVariables == nil {
			continue
		}

		env.LoadOutputVariables(node.inner.State.OutputVariables)
	}

	return WithEnv(ctx, env)
}

func (r *Runner) execNode(ctx context.Context, node *Node) error {
	if !r.dry {
		if err := node.Execute(ctx); err != nil {
			return fmt.Errorf("failed to execute step %q: %w", node.Name(), err)
		}
	}

	return nil
}

// Signal sends a signal to the runner.
// for a node with repeat policy, it does not stop the node and
// wait to finish current run.
func (r *Runner) Signal(
	ctx context.Context, plan *Plan, sig os.Signal, done chan bool, allowOverride bool,
) {
	for _, node := range plan.Nodes() {
		// for a repetitive task, we'll wait for the job to finish
		// until time reaches max wait time
		if node.Step().RepeatPolicy.RepeatMode != "" {
			logger.Info(ctx, "Waiting for repeat node to finish",
				tag.Step(node.Step().Name),
			)
			continue
		}
		node.Signal(ctx, sig, allowOverride)
	}

	isTermination := signal.IsTerminationSignalOS(sig)
	if !r.isCanceled() && isTermination {
		r.setCanceled()
	}

	if done != nil && isTermination {
		defer func() {
			for plan.IsRunning() {
				time.Sleep(r.pause)
			}
			done <- true
		}()
	}
}

// Cancel sends -1 signal to all nodes.
func (r *Runner) Cancel(p *Plan) {
	r.setCanceled()
	for _, node := range p.Nodes() {
		node.Cancel()
	}
}

// Status returns the status of the runner.
func (r *Runner) Status(ctx context.Context, p *Plan) core.Status {
	if r.isCanceled() && !r.isSucceed(p) {
		return core.Aborted
	}
	if !p.IsStarted() {
		return core.NotStarted
	}
	if p.IsRunning() {
		return core.Running
	}

	if r.isPartialSuccess(ctx, p) {
		return core.PartiallySucceeded
	}

	if r.isError() {
		return core.Failed
	}

	return core.Succeeded
}

func (r *Runner) isError() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastError != nil
}

// HandlerNode returns the handler node with the given name.
func (r *Runner) HandlerNode(name core.HandlerType) *Node {
	r.handlerMu.RLock()
	defer r.handlerMu.RUnlock()
	if v, ok := r.handlers[name]; ok {
		return v
	}
	return nil
}

// isCanceled returns true if the runner is canceled.
func (r *Runner) isCanceled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.canceled == 1
}

func isReady(ctx context.Context, plan *Plan, node *Node) bool {
	ready := true
	for _, depID := range plan.Dependencies(node.id) {
		dep := plan.GetNode(depID)

		switch dep.State().Status {
		case core.NodeSucceeded:
			continue

		case core.NodePartiallySucceeded:
			// Partial success is treated like success for dependencies
			continue

		case core.NodeFailed:
			if dep.ShouldContinue(ctx) {
				logger.Debug(ctx, "Dependency failed but allowed to continue",
					tag.Step(node.Name()),
					tag.Dependency(dep.Name()),
				)
				continue
			}
			logger.Debug(ctx, "Dependency failed",
				tag.Step(node.Name()),
				tag.Dependency(dep.Name()),
			)
			ready = false
			node.SetStatus(core.NodeAborted)
			node.SetError(ErrUpstreamFailed)

		case core.NodeSkipped:
			if dep.ShouldContinue(ctx) {
				logger.Debug(ctx, "Dependency skipped but allowed to continue",
					tag.Step(node.Name()),
					tag.Dependency(dep.Name()),
				)
				continue
			}
			logger.Debug(ctx, "Dependency skipped",
				tag.Step(node.Name()),
				tag.Dependency(dep.Name()),
			)
			ready = false
			node.SetStatus(core.NodeSkipped)
			node.SetError(ErrUpstreamSkipped)

		case core.NodeAborted:
			logger.Debug(ctx, "Dependency aborted",
				tag.Step(node.Name()),
				tag.Dependency(dep.Name()),
			)
			ready = false
			node.SetStatus(core.NodeAborted)

		case core.NodeNotStarted, core.NodeRunning:
			logger.Debug(ctx, "Dependency not finished",
				tag.Step(node.Name()),
				tag.Dependency(dep.Name()),
				tag.Status(dep.State().Status.String()),
			)
			ready = false

		default:
			ready = false

		}
	}
	return ready
}

func (r *Runner) runEventHandler(ctx context.Context, plan *Plan, node *Node) error {
	defer node.Finish()

	if !r.dry {
		if err := node.Prepare(ctx, r.logDir, r.dagRunID); err != nil {
			node.SetStatus(core.NodeFailed)
			return nil
		}

		defer func() {
			_ = node.Teardown()
		}()

		node.SetStatus(core.NodeRunning)

		ctx = r.setupEnvironEventHandler(ctx, plan, node)
		if err := node.Execute(ctx); err != nil {
			node.SetStatus(core.NodeFailed)
			return err
		}

		node.SetStatus(core.NodeSucceeded)
	} else {
		node.SetStatus(core.NodeSucceeded)
	}

	return nil
}

func (r *Runner) setup(ctx context.Context) (err error) {
	if !r.dry {
		if err = os.MkdirAll(r.logDir, 0750); err != nil {
			err = fmt.Errorf("failed to create log directory: %w", err)
			return err
		}
	}

	// Initialize handlers
	r.handlerMu.Lock()
	defer r.handlerMu.Unlock()
	r.handlers = map[core.HandlerType]*Node{}
	if r.onExit != nil {
		r.handlers[core.HandlerOnExit] = &Node{Data: newSafeData(NodeData{Step: *r.onExit})}
	}
	if r.onSuccess != nil {
		r.handlers[core.HandlerOnSuccess] = &Node{Data: newSafeData(NodeData{Step: *r.onSuccess})}
	}
	if r.onFailure != nil {
		r.handlers[core.HandlerOnFailure] = &Node{Data: newSafeData(NodeData{Step: *r.onFailure})}
	}
	if r.onCancel != nil {
		r.handlers[core.HandlerOnCancel] = &Node{Data: newSafeData(NodeData{Step: *r.onCancel})}
	}

	// Initialize metrics
	r.metrics.startTime = time.Now()

	// Log runner setup
	logger.Debug(ctx, "Runner setup complete",
		slog.String("dagRunId", r.dagRunID),
		slog.Int("maxActiveRuns", r.maxActiveRuns),
		slog.Duration("timeout", r.timeout),
		slog.Bool("dry", r.dry),
	)

	return err
}

func (r *Runner) setCanceled() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.canceled = 1
}

func (r *Runner) isSucceed(p *Plan) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, node := range p.Nodes() {
		nodeStatus := node.State().Status
		if nodeStatus == core.NodeSucceeded || nodeStatus == core.NodeSkipped || nodeStatus == core.NodePartiallySucceeded {
			continue
		}
		return false
	}
	return true
}

// isPartialSuccess checks if the DAG completed with some failures that were allowed to continue.
// This represents scenarios where execution continued despite failures due to continueOn conditions.
func (r *Runner) isPartialSuccess(ctx context.Context, p *Plan) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hasFailuresWithContinueOn := false
	hasSuccessfulNodes := false

	// First pass: check if any failed node is NOT allowed to continue
	// If so, this is an error, not partial success
	for _, node := range p.Nodes() {
		if node.State().Status == core.NodeFailed {
			if !node.ShouldContinue(ctx) {
				// Found a failed node that was NOT allowed to continue
				// This disqualifies the DAG from being partial success
				return false
			}
		}
	}

	// Second pass: check for partial success conditions
	for _, node := range p.Nodes() {
		switch node.State().Status {
		case core.NodeSucceeded:
			hasSuccessfulNodes = true
		case core.NodeFailed:
			if node.ShouldContinue(ctx) && !node.ShouldMarkSuccess(ctx) {
				hasFailuresWithContinueOn = true
			}
		case core.NodePartiallySucceeded:
			// Partial success at node level contributes to overall partial success
			hasFailuresWithContinueOn = true
			hasSuccessfulNodes = true
		case core.NodeNotStarted, core.NodeRunning, core.NodeAborted, core.NodeSkipped:
			// These statuses don't affect partial success determination, but are needed for linter
		}
	}

	// Partial success requires:
	// 1. At least one failed node with continueOn (some non-critical failures)
	// 2. No failed nodes without continueOn (checked in first pass)
	// Note: Skipped nodes alone do not count as successful completion
	return hasSuccessfulNodes && hasFailuresWithContinueOn
}

func (r *Runner) isTimeout(startedAt time.Time) bool {
	return r.timeout > 0 && time.Since(startedAt) > r.timeout
}

// GetMetrics returns the current metrics for the runner
func (r *Runner) GetMetrics() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := map[string]any{
		"totalNodes":         r.metrics.totalNodes,
		"completedNodes":     r.metrics.completedNodes,
		"failedNodes":        r.metrics.failedNodes,
		"skippedNodes":       r.metrics.skippedNodes,
		"canceledNodes":      r.metrics.canceledNodes,
		"totalExecutionTime": r.metrics.totalExecutionTime.String(),
	}

	return metrics
}

// shouldRetryNode handles the retry logic for a node based on exit codes and retry policy
func (r *Runner) shouldRetryNode(ctx context.Context, node *Node, execErr error) (shouldRetry bool) {
	exitCode := 1
	if code, found := exitCodeFromError(execErr); found {
		exitCode = code
		logger.Debug(ctx, "Resolved exit code from error",
			tag.Error(execErr),
			tag.ExitCode(exitCode),
		)
	} else {
		logger.Debug(ctx, "Could not determine exit code",
			tag.Error(execErr),
			slog.String("error-type", fmt.Sprintf("%T", execErr)),
		)
	}

	shouldRetry = node.retryPolicy.ShouldRetry(exitCode)
	logger.Debug(ctx, "Checking retry policy",
		tag.ExitCode(exitCode),
		slog.Any("allowed-codes", node.retryPolicy.ExitCodes),
		slog.Bool("should-retry", shouldRetry),
	)

	if !shouldRetry {
		// finish the node with error
		node.SetStatus(core.NodeFailed)
		node.MarkError(execErr)
		r.setLastError(execErr)
		return false
	}

	logger.Info(ctx, "Step execution failed; retrying",
		tag.Error(execErr),
		slog.Int("retry", node.GetRetryCount()),
		tag.ExitCode(exitCode),
	)

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
	node.SetStatus(core.NodeRunning)
	return true
}

// recoverNodePanic handles panic recovery for a node goroutine.
func (r *Runner) recoverNodePanic(ctx context.Context, node *Node) {
	if panicObj := recover(); panicObj != nil {
		stack := string(debug.Stack())
		err := fmt.Errorf("panic recovered in node %s: %v\n%s", node.Name(), panicObj, stack)
		logger.Error(ctx, "Panic occurred",
			tag.Error(err),
			slog.String("stack", stack),
			tag.RunID(r.dagRunID),
		)
		node.MarkError(err)
		r.setLastError(err)

		// Update metrics for failed node
		r.mu.Lock()
		r.metrics.failedNodes++
		r.mu.Unlock()
	}
}

// finishNode updates metrics and finalizes the node.
func (r *Runner) finishNode(node *Node, wg *sync.WaitGroup) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch node.State().Status {
	case core.NodeSucceeded:
		r.metrics.completedNodes++
	case core.NodeFailed:
		r.metrics.failedNodes++
	case core.NodeSkipped:
		r.metrics.skippedNodes++
	case core.NodeAborted:
		r.metrics.canceledNodes++
	case core.NodePartiallySucceeded:
		r.metrics.completedNodes++ // Count partial success as completed
	case core.NodeNotStarted, core.NodeRunning:
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
		node.SetStatus(core.NodeSkipped)
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
// handleNodeExecutionError handles the error from node execution and determines if it should be retried.
func (r *Runner) handleNodeExecutionError(ctx context.Context, plan *Plan, node *Node, execErr error) bool {
	if execErr == nil {
		return false // no error, nothing to handle
	}

	s := node.State().Status
	switch {
	case s == core.NodeSucceeded || s == core.NodeAborted || s == core.NodePartiallySucceeded:
		// do nothing

	// Check for timeout errors first (both step-level and DAG-level)
	case errors.Is(execErr, context.DeadlineExceeded):
		step := node.Step()
		if step.Timeout > 0 {
			// Step-level timeout: Node.Execute already set status to failed and exitCode=124.
			// Keep failed status and ensure we don't retry.
			logger.Info(ctx, "Step timed out (step-level timeout)",
				tag.Timeout(step.Timeout),
				tag.Error(execErr),
			)
			// Ensure status is failed (in case earlier logic differed)
			node.SetStatus(core.NodeFailed)
		} else if r.isTimeout(plan.StartAt()) {
			// DAG-level timeout -> treat as aborted (global cancellation semantics)
			logger.Info(ctx, "Step deadline exceeded (DAG-level timeout)",
				tag.Timeout(r.timeout),
				tag.Error(execErr),
			)
			node.SetStatus(core.NodeAborted)
		} else {
			// Parent context canceled or other deadline; mark aborted for safety
			logger.Info(ctx, "Step deadline exceeded", tag.Error(execErr))
			node.SetStatus(core.NodeAborted)
		}
		r.setLastError(execErr)

	case r.isTimeout(plan.StartAt()):
		// DAG-level timeout (non-context error case)
		logger.Info(ctx, "Step deadline exceeded (DAG-level timeout)",
			tag.Timeout(r.timeout),
			tag.Error(execErr),
		)
		node.SetStatus(core.NodeAborted)
		r.setLastError(execErr)

	case r.isCanceled():
		r.setLastError(execErr)

	case node.retryPolicy.Limit > node.GetRetryCount():
		if r.shouldRetryNode(ctx, node, execErr) {
			return true
		}

	default:
		// node execution error is unexpected and unrecoverable
		node.SetStatus(core.NodeFailed)
		if node.ShouldMarkSuccess(ctx) {
			// mark as success if the node should be force marked as success
			// i.e. continueOn.markSuccess is set to true
			node.SetStatus(core.NodeSucceeded)
		} else {
			node.MarkError(execErr)
			r.setLastError(execErr)
		}
	}

	return false
}

// shouldRepeatNode determines if a node should be repeated based on its repeat policy
func (r *Runner) shouldRepeatNode(ctx context.Context, node *Node, execErr error) bool {
	step := node.Step()
	rp := step.RepeatPolicy

	// First, check the hard limit. This overrides everything.
	if rp.Limit > 0 && node.State().DoneCount >= rp.Limit {
		return false
	}

	env := GetEnv(ctx)
	shell := env.Shell(ctx)
	switch rp.RepeatMode {
	case core.RepeatModeWhile:
		// It's a 'while' loop. Repeat while a condition is true.
		if rp.Condition != nil {
			// Ensure node's own output variables are reloaded before evaluating the condition.
			if node.inner.State.OutputVariables != nil {
				env := GetEnv(ctx)
				env.ForceLoadOutputVariables(node.inner.State.OutputVariables)
				ctx = WithEnv(ctx, env)
			}
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
	case core.RepeatModeUntil:
		// It's an 'until' loop. Repeat until a condition is true.
		if rp.Condition != nil {
			// Ensure node's own output variables are reloaded before evaluating the condition.
			if node.inner.State.OutputVariables != nil {
				env := GetEnv(ctx)
				env.ForceLoadOutputVariables(node.inner.State.OutputVariables)
				ctx = WithEnv(ctx, env)
			}
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
func (r *Runner) prepareNodeForRepeat(ctx context.Context, node *Node, progressCh chan *Node) {
	step := node.Step()

	node.SetStatus(core.NodeRunning) // reset status to running for the repeat
	if r.lastError == node.Error() {
		r.setLastError(nil) // clear last error if we are repeating
	}
	logger.Info(ctx, "Step will be repeated",
		slog.Duration("interval", step.RepeatPolicy.Interval),
	)
	interval := calculateBackoffInterval(
		step.RepeatPolicy.Interval,
		step.RepeatPolicy.Backoff,
		step.RepeatPolicy.MaxInterval,
		node.State().DoneCount,
	)
	time.Sleep(interval)
	node.SetRepeated(true) // mark as repeated
	logger.Info(ctx, "Repeating step")

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

func NewPlanEnv(ctx context.Context, step core.Step, plan *Plan) Env {
	env := NewEnv(ctx, step)
	for _, n := range plan.Nodes() {
		if n.Step().ID != "" {
			env.StepMap[n.Step().ID] = n.StepInfo()
		}
	}
	return env
}

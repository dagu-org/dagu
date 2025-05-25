package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/logger"
)

type Status int

const (
	StatusNone Status = iota
	StatusRunning
	StatusError
	StatusCancel
	StatusSuccess
	StatusQueued
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
	case StatusNone:
		fallthrough
	default:
		return "not started"
	}
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
	workflowID    string

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
		workflowID:    cfg.WorkflowID,
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
	WorkflowID     string
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

	for !sc.isFinished(graph) {
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
			if sc.maxActiveRuns > 0 && sc.runningCount(graph) >= sc.maxActiveRuns {
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
				defer func() {
					if panicObj := recover(); panicObj != nil {
						stack := string(debug.Stack())
						err := fmt.Errorf("panic recovered in node %s: %v\n%s", node.Name(), panicObj, stack)
						logger.Error(ctx, "Panic occurred",
							"error", err,
							"step", node.Name(),
							"stack", stack,
							"workflowId", sc.workflowID)
						node.MarkError(err)
						sc.setLastError(err)

						// Update metrics for failed node
						sc.mu.Lock()
						sc.metrics.failedNodes++
						sc.mu.Unlock()
					}
				}()

				// Ensure node is finished and wg is decremented
				defer func() {
					// Update metrics based on node status
					sc.mu.Lock()

					// Update node status counts
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
					sc.mu.Unlock()

					node.Finish()
					wg.Done()
				}()

				ctx = sc.setupEnviron(nodeCtx, graph, node)

				// Check preconditions
				if len(node.Step().Preconditions) > 0 {
					logger.Infof(ctx, "Checking preconditions for \"%s\"", node.Name())
					shell := cmdutil.GetShellCommand(node.Step().Shell)
					if err := EvalConditions(ctx, shell, node.Step().Preconditions); err != nil {
						logger.Infof(ctx, "Preconditions failed for \"%s\"", node.Name())
						node.SetStatus(NodeStatusSkipped)
						if !errors.Is(err, ErrConditionNotMet) {
							node.SetError(err)
						}
						if progressCh != nil {
							progressCh <- node
						}
						return
					}
				}

				setupSucceed := true
				if err := sc.setupNode(ctx, node); err != nil {
					setupSucceed = false
					sc.setLastError(err)
					node.MarkError(err)
				}

				ctx = node.SetupContextBeforeExec(ctx)

				defer func() {
					_ = sc.teardownNode(ctx, node)
				}()

			ExecRepeat: // repeat execution
				for setupSucceed && !sc.isCanceled() {
					execErr := sc.execNode(ctx, node)
					if execErr != nil {
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
							if sc.handleNodeRetry(ctx, node, execErr) {
								continue ExecRepeat
							}

						default:
							// finish the node
							node.SetStatus(NodeStatusError)
							if node.shouldMarkSuccess(ctx) {
								// mark as success if the node should be marked as success
								// i.e. continueOn.markSuccess is set to true
								node.SetStatus(NodeStatusSuccess)
							} else {
								node.MarkError(execErr)
								sc.setLastError(execErr)
							}
						}
					}

					if node.State().Status != NodeStatusCancel {
						node.IncDoneCount()
					}

					shouldRepeat := false
					step := node.Step()
					if step.RepeatPolicy.Condition != "" {
						cond := step.RepeatPolicy.Condition
						expected := step.RepeatPolicy.Expected
						if strings.HasPrefix(cond, "`") && strings.HasSuffix(cond, "`") && len(cond) > 2 {
							// Command substitution: run the command, compare output or exit code
							cmdStr := cond[1 : len(cond)-1]
							cmdOut, cmdExit, err := "", 0, error(nil)
							if out, exit, e := runShellCommandWithExitCode(ctx, cmdStr, step.Shell); true {
								cmdOut, cmdExit, err = out, exit, e
							}
							if err == nil {
								if expected != "" {
									shouldRepeat = (strings.TrimSpace(cmdOut) != expected)
								} else {
									shouldRepeat = (cmdExit != 0)
								}
							} else {
								shouldRepeat = true // repeat on error
							}
						} else {
							// Simple string match: compare last step output to expected
							output := node.LastOutput()
							shouldRepeat = (expected != "" && strings.TrimSpace(output) != expected)
						}
					} else if len(step.RepeatPolicy.ExitCode) > 0 {
						// Repeat if last exit code matches any in ExitCode
						lastExit := node.State().ExitCode
						for _, code := range step.RepeatPolicy.ExitCode {
							if lastExit == code {
								shouldRepeat = true
								break
							}
						}
					} else if step.RepeatPolicy.Repeat {
						// Legacy unconditional repeat
						shouldRepeat = (execErr == nil || step.ContinueOn.Failure)
					}

					if shouldRepeat && !sc.isCanceled() {
						time.Sleep(step.RepeatPolicy.Interval)
						if progressCh != nil {
							progressCh <- node
						}
						continue
					}

					if execErr != nil && progressCh != nil {
						progressCh <- node
						return
					}

					break ExecRepeat
				}

				// finish the node
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
	switch sc.Status(graph) {
	case StatusSuccess:
		eventHandlers = append(eventHandlers, digraph.HandlerOnSuccess)

	case StatusError:
		eventHandlers = append(eventHandlers, digraph.HandlerOnFailure)

	case StatusCancel:
		eventHandlers = append(eventHandlers, digraph.HandlerOnCancel)

	case StatusNone, StatusRunning, StatusQueued:
		// These states should not occur at this point
		logger.Warn(ctx, "Unexpected final status",
			"status", sc.Status(graph).String(),
			"workflowId", sc.workflowID)
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

	return sc.lastError
}

func (sc *Scheduler) setLastError(err error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.lastError = err
}

func (sc *Scheduler) setupNode(ctx context.Context, node *Node) error {
	if !sc.dry {
		return node.Setup(ctx, sc.logDir, sc.workflowID)
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

	curr := node.id
	visited := make(map[int]struct{})
	queue := []int{curr}
	for len(queue) > 0 {
		curr, queue = queue[0], queue[1:]
		if _, ok := visited[curr]; ok {
			continue
		}
		visited[curr] = struct{}{}
		queue = append(queue, graph.To[curr]...)

		node := graph.nodeByID[curr]
		if node.inner.State.OutputVariables == nil {
			continue
		}

		env.LoadOutputVariables(node.inner.State.OutputVariables)
	}

	return executor.WithEnv(ctx, env)
}

func (sc *Scheduler) setupEnvironEventHandler(ctx context.Context, graph *ExecutionGraph, node *Node) context.Context {
	env := executor.NewEnv(ctx, node.Step())

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
func (sc *Scheduler) Status(g *ExecutionGraph) Status {
	if sc.isCanceled() && !sc.isSucceed(g) {
		return StatusCancel
	}
	if !g.IsStarted() {
		return StatusNone
	}
	if g.IsRunning() {
		return StatusRunning
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
			if dep.shouldContinue(ctx) {
				continue
			}
			ready = false
			node.SetStatus(NodeStatusCancel)
			node.SetError(ErrUpstreamFailed)

		case NodeStatusSkipped:
			if dep.shouldContinue(ctx) {
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
		if err := node.Setup(ctx, sc.logDir, sc.workflowID); err != nil {
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
		"workflowId", sc.workflowID,
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

func (*Scheduler) runningCount(g *ExecutionGraph) int {
	count := 0
	for _, node := range g.nodes {
		if node.State().Status == NodeStatusRunning {
			count++
		}
	}
	return count
}

func (*Scheduler) isFinished(g *ExecutionGraph) bool {
	for _, node := range g.nodes {
		if node.State().Status == NodeStatusRunning ||
			node.State().Status == NodeStatusNone {
			return false
		}
	}
	return true
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

// handleNodeRetry handles the retry logic for a node based on exit codes and retry policy
func (sc *Scheduler) handleNodeRetry(ctx context.Context, node *Node, execErr error) (shouldRetry bool) {
	var exitCode int
	var exitCodeFound bool

	// Try to extract exit code from different error types
	if exitErr, ok := execErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		exitCodeFound = true
		logger.Debug(ctx, "Found exit error", "error", execErr, "exitCode", exitCode)
	} else {
		// Try to parse exit code from error string
		errStr := execErr.Error()
		if strings.Contains(errStr, "exit status") {
			// Parse "exit status N" format
			parts := strings.Split(errStr, " ")
			if len(parts) > 2 {
				if code, err := strconv.Atoi(parts[2]); err == nil {
					exitCode = code
					exitCodeFound = true
					logger.Debug(ctx, "Parsed exit code from error string", "error", errStr, "exitCode", exitCode)
				}
			}
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
	node.SetStatus(NodeStatusNone)
	return true
}

// Helper for repeatPolicy: run a shell command and get output and exit code
func runShellCommandWithExitCode(ctx context.Context, cmdStr string, shell string) (string, int, error) {
	sh := shell
	if sh == "" {
		sh = "/bin/sh"
	}
	cmd := exec.CommandContext(ctx, sh, "-c", cmdStr)
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return outBuf.String(), exitCode, err
}

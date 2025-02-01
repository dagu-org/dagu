package scheduler

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
)

type Status int

const (
	StatusNone Status = iota
	StatusRunning
	StatusError
	StatusCancel
	StatusSuccess
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
	case StatusNone:
		fallthrough
	default:
		return "not started"
	}
}

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
	requestID     string

	canceled  int32
	mu        sync.RWMutex
	pause     time.Duration
	lastError error
	handlers  map[digraph.HandlerType]*Node
}

func New(cfg *Config) *Scheduler {
	return &Scheduler{
		logDir:        cfg.LogDir,
		maxActiveRuns: cfg.MaxActiveRuns,
		timeout:       cfg.Timeout,
		delay:         cfg.Delay,
		dry:           cfg.Dry,
		onExit:        cfg.OnExit,
		onSuccess:     cfg.OnSuccess,
		onFailure:     cfg.OnFailure,
		onCancel:      cfg.OnCancel,
		requestID:     cfg.ReqID,
		pause:         time.Millisecond * 100,
	}
}

type Config struct {
	LogDir        string
	MaxActiveRuns int
	Timeout       time.Duration
	Delay         time.Duration
	Dry           bool
	OnExit        *digraph.Step
	OnSuccess     *digraph.Step
	OnFailure     *digraph.Step
	OnCancel      *digraph.Step
	ReqID         string
}

// Schedule runs the graph of steps.
func (sc *Scheduler) Schedule(ctx context.Context, graph *ExecutionGraph, done chan *Node) error {
	if err := sc.setup(ctx); err != nil {
		return err
	}
	graph.Start()
	defer graph.Finish()

	var wg = sync.WaitGroup{}

	var cancel context.CancelFunc
	if sc.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, sc.timeout)
		defer cancel()
	}

	for !sc.isFinished(graph) {
		if sc.isCanceled() {
			break
		}

	NodesIteration:
		for _, node := range graph.Nodes() {
			if node.State().Status != NodeStatusNone || !isReady(ctx, graph, node) {
				continue NodesIteration
			}
			if sc.isCanceled() {
				break NodesIteration
			}
			if sc.maxActiveRuns > 0 && sc.runningCount(graph) >= sc.maxActiveRuns {
				continue NodesIteration
			}

			// Check preconditions
			if len(node.data.Step.Preconditions) > 0 {
				logger.Infof(ctx, "Checking pre conditions for \"%s\"", node.data.Step.Name)
				if err := digraph.EvalConditions(ctx, node.data.Step.Preconditions); err != nil {
					logger.Infof(ctx, "Pre conditions failed for \"%s\"", node.data.Step.Name)
					node.SetStatus(NodeStatusSkipped)
					node.setError(err)
					continue NodesIteration
				}
			}

			wg.Add(1)

			logger.Info(ctx, "Step execution started", "step", node.data.Step.Name)
			node.SetStatus(NodeStatusRunning)
			go func(ctx context.Context, node *Node) {
				defer func() {
					if panicObj := recover(); panicObj != nil {
						stack := string(debug.Stack())
						err := fmt.Errorf("panic recovered: %v\n%s", panicObj, stack)
						logger.Error(ctx, "Panic occurred", "error", err, "step", node.data.Step.Name, "stack", stack)
						node.MarkError(err)
						sc.setLastError(err)
					}
				}()

				defer func() {
					node.Finish()
					wg.Done()
				}()

				ctx = sc.setupContext(ctx, graph, node)

				setupSucceed := true
				if err := sc.setupNode(ctx, node); err != nil {
					setupSucceed = false
					sc.setLastError(err)
					node.MarkError(err)
				}

				ctx = node.SetupContextBeforeExec(ctx)

				defer func() {
					_ = sc.teardownNode(node)
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
							logger.Info(ctx, "Step execution deadline exceeded", "step", node.data.Step.Name, "error", execErr)
							node.SetStatus(NodeStatusCancel)
							sc.setLastError(execErr)

						case sc.isCanceled():
							sc.setLastError(execErr)

						case node.retryPolicy.Limit > node.GetRetryCount():
							// retry
							node.IncRetryCount()
							logger.Info(ctx, "Step execution failed. Retrying...", "step", node.data.Step.Name, "error", execErr, "retry", node.GetRetryCount())
							time.Sleep(node.retryPolicy.Interval)
							node.SetRetriedAt(time.Now())
							node.SetStatus(NodeStatusNone)

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

					if node.data.Step.RepeatPolicy.Repeat {
						if execErr == nil || node.data.Step.ContinueOn.Failure {
							if !sc.isCanceled() {
								time.Sleep(node.data.Step.RepeatPolicy.Interval)
								if done != nil {
									done <- node
								}
								continue ExecRepeat
							}
						}
					}

					if execErr != nil && done != nil {
						done <- node
						return
					}

					break ExecRepeat
				}

				// finish the node
				if node.State().Status == NodeStatusRunning {
					node.SetStatus(NodeStatusSuccess)
				}

				if err := sc.teardownNode(node); err != nil {
					sc.setLastError(err)
					node.SetStatus(NodeStatusError)
				}

				if done != nil {
					done <- node
				}
			}(ctx, node)

			time.Sleep(sc.delay) // TODO: check if this is necessary
		}

		time.Sleep(sc.pause) // avoid busy loop
	}

	wg.Wait()

	var handlers []digraph.HandlerType
	switch sc.Status(graph) {
	case StatusSuccess:
		handlers = append(handlers, digraph.HandlerOnSuccess)

	case StatusError:
		handlers = append(handlers, digraph.HandlerOnFailure)

	case StatusCancel:
		handlers = append(handlers, digraph.HandlerOnCancel)

	case StatusNone:
		// do nothing (should not happen)

	case StatusRunning:
		// do nothing (should not happen)

	}

	handlers = append(handlers, digraph.HandlerOnExit)
	for _, handler := range handlers {
		if handlerNode := sc.handlers[handler]; handlerNode != nil {
			logger.Info(ctx, "Handler execution started", "handler", handlerNode.data.Step.Name)
			if err := sc.runHandlerNode(ctx, graph, handlerNode); err != nil {
				sc.setLastError(err)
			}

			if done != nil {
				done <- handlerNode
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
		return node.Setup(ctx, sc.logDir, sc.requestID)
	}
	return nil
}

func (sc *Scheduler) teardownNode(node *Node) error {
	if !sc.dry {
		return node.Teardown()
	}
	return nil
}

// setupContext builds the context for a step.
func (sc *Scheduler) setupContext(ctx context.Context, graph *ExecutionGraph, node *Node) context.Context {
	stepCtx := digraph.NewStepContext(ctx, node.data.Step)

	// get output variables that are available to the next steps
	curr := node.id
	visited := make(map[int]struct{})
	queue := []int{curr}
	for len(queue) > 0 {
		curr, queue = queue[0], queue[1:]
		if _, ok := visited[curr]; ok {
			continue
		}
		visited[curr] = struct{}{}
		queue = append(queue, graph.to[curr]...)

		node := graph.node(curr)
		if node.data.Step.OutputVariables == nil {
			continue
		}

		stepCtx.LoadOutputVariables(node.data.Step.OutputVariables)
	}

	return digraph.WithStepContext(ctx, stepCtx)
}

// buildStepContextForHandler builds the context for a handler.
func (sc *Scheduler) buildStepContextForHandler(ctx context.Context, graph *ExecutionGraph, node *Node) context.Context {
	stepCtx := digraph.NewStepContext(ctx, node.data.Step)

	// get all output variables
	for _, node := range graph.Nodes() {
		if node.data.Step.OutputVariables == nil {
			continue
		}

		stepCtx.LoadOutputVariables(node.data.Step.OutputVariables)
	}

	return digraph.WithStepContext(ctx, stepCtx)
}

func (sc *Scheduler) execNode(ctx context.Context, node *Node) error {
	if !sc.dry {
		if err := node.Execute(ctx); err != nil {
			return fmt.Errorf("failed to execute step %q: %w", node.data.Step.Name, err)
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

	for _, node := range graph.Nodes() {
		// for a repetitive task, we'll wait for the job to finish
		// until time reaches max wait time
		if !node.data.Step.RepeatPolicy.Repeat {
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
	for _, node := range g.Nodes() {
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
	for _, dep := range g.to[node.id] {
		dep := g.node(dep)

		switch dep.State().Status {
		case NodeStatusSuccess:
			continue

		case NodeStatusError:
			if dep.shouldContinue(ctx) {
				continue
			}
			ready = false
			node.SetStatus(NodeStatusCancel)
			node.setError(errUpstreamFailed)

		case NodeStatusSkipped:
			if dep.shouldContinue(ctx) {
				continue
			}
			ready = false
			node.SetStatus(NodeStatusSkipped)
			node.setError(errUpstreamSkipped)

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

func (sc *Scheduler) runHandlerNode(ctx context.Context, graph *ExecutionGraph, node *Node) error {
	defer func() {
		node.data.State.FinishedAt = time.Now()
	}()

	node.SetStatus(NodeStatusRunning)

	if !sc.dry {
		if err := node.Setup(ctx, sc.logDir, sc.requestID); err != nil {
			node.SetStatus(NodeStatusError)
			return nil
		}

		defer func() {
			_ = node.Teardown()
		}()

		ctx = sc.buildStepContextForHandler(ctx, graph, node)
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
	digraph.GetContext(ctx).ApplyEnvs()

	if !sc.dry {
		if err = os.MkdirAll(sc.logDir, 0755); err != nil {
			err = fmt.Errorf("failed to create log directory: %w", err)
			return err
		}
	}

	sc.handlers = map[digraph.HandlerType]*Node{}
	if sc.onExit != nil {
		sc.handlers[digraph.HandlerOnExit] = &Node{data: NodeData{Step: *sc.onExit}}
	}
	if sc.onSuccess != nil {
		sc.handlers[digraph.HandlerOnSuccess] =
			&Node{data: NodeData{Step: *sc.onSuccess}}
	}
	if sc.onFailure != nil {
		sc.handlers[digraph.HandlerOnFailure] =
			&Node{data: NodeData{Step: *sc.onFailure}}
	}
	if sc.onCancel != nil {
		sc.handlers[digraph.HandlerOnCancel] =
			&Node{data: NodeData{Step: *sc.onCancel}}
	}

	return err
}

func (sc *Scheduler) setCanceled() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.canceled = 1
}

func (*Scheduler) runningCount(g *ExecutionGraph) int {
	count := 0
	for _, node := range g.Nodes() {
		if node.State().Status == NodeStatusRunning {
			count++
		}
	}
	return count
}

func (*Scheduler) isFinished(g *ExecutionGraph) bool {
	for _, node := range g.Nodes() {
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
	for _, node := range g.Nodes() {
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

var (
	errUpstreamFailed  = fmt.Errorf("upstream failed")
	errUpstreamSkipped = fmt.Errorf("upstream skipped")
)

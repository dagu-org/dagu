package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
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
	*Config

	canceled  int32
	mu        sync.RWMutex
	pause     time.Duration
	lastError error
	handlers  map[dag.HandlerType]*Node
}

func New(cfg *Config) *Scheduler {
	return &Scheduler{Config: cfg}
}

type Config struct {
	LogDir        string
	MaxActiveRuns int
	Delay         time.Duration
	Dry           bool
	OnExit        *dag.Step
	OnSuccess     *dag.Step
	OnFailure     *dag.Step
	OnCancel      *dag.Step
	ReqID         string
}

// Schedule runs the graph of steps.
// nolint // cognitive complexity
func (sc *Scheduler) Schedule(ctx context.Context, g *ExecutionGraph, done chan *Node) error {
	if err := sc.setup(); err != nil {
		return err
	}
	g.Start()
	defer g.Finish()

	var wg = sync.WaitGroup{}

	for !sc.isFinished(g) {
		if sc.isCanceled() {
			break
		}
	NodesIteration:
		for _, node := range g.Nodes() {
			if node.State().Status != NodeStatusNone || !isReady(g, node) {
				continue NodesIteration
			}
			if sc.isCanceled() {
				break NodesIteration
			}
			if sc.MaxActiveRuns > 0 && sc.runningCount(g) >= sc.MaxActiveRuns {
				continue NodesIteration
			}
			// Check preconditions
			if len(node.data.Step.Preconditions) > 0 {
				log.Printf("checking pre conditions for \"%s\"", node.data.Step.Name)
				if err := dag.EvalConditions(node.data.Step.Preconditions); err != nil {
					log.Printf("%s", err.Error())
					node.setStatus(NodeStatusSkipped)
					node.SetError(err)
					continue NodesIteration
				}
			}
			wg.Add(1)

			log.Printf("start running: %s", node.data.Step.Name)
			node.setStatus(NodeStatusRunning)
			go func(node *Node) {
				defer func() {
					node.finish()
					wg.Done()
				}()

				setupSucceed := true
				if err := sc.setupNode(node); err != nil {
					setupSucceed = false
					sc.lastError = err
					node.setErr(err)
				}
				defer func() {
					_ = sc.teardownNode(node)
				}()

			ExecRepeat:
				for setupSucceed && !sc.isCanceled() {
					execErr := sc.execNode(ctx, node)
					if execErr != nil {
						status := node.State().Status
						switch {
						case status == NodeStatusSuccess || status == NodeStatusCancel:
							// do nothing
						case sc.isCanceled():
							sc.setLastError(execErr)
						case node.data.Step.RetryPolicy != nil && node.data.Step.RetryPolicy.Limit > node.getRetryCount():
							// retry
							log.Printf("%s failed but scheduled for retry", node.data.Step.Name)
							node.incRetryCount()
							log.Printf("sleep %s for retry", node.data.Step.RetryPolicy.Interval)
							time.Sleep(node.data.Step.RetryPolicy.Interval)
							node.setRetriedAt(time.Now())
							node.setStatus(NodeStatusNone)
						default:
							// finish the node
							node.setStatus(NodeStatusError)
							node.setErr(execErr)
							sc.setLastError(execErr)
						}
					}
					if node.State().Status != NodeStatusCancel {
						node.incDoneCount()
					}
					if node.data.Step.RepeatPolicy.Repeat {
						if execErr == nil || node.data.Step.ContinueOn.Failure {
							if !sc.isCanceled() {
								time.Sleep(node.data.Step.RepeatPolicy.Interval)
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
					node.setStatus(NodeStatusSuccess)
				}
				if err := sc.teardownNode(node); err != nil {
					sc.setLastError(err)
					node.setStatus(NodeStatusError)
				}
				if done != nil {
					done <- node
				}
			}(node)
			time.Sleep(sc.Delay)
		}
		time.Sleep(sc.pause)
	}
	wg.Wait()

	var handlers []dag.HandlerType
	switch sc.Status(g) {
	case StatusSuccess:
		handlers = append(handlers, dag.HandlerOnSuccess)
	case StatusError:
		handlers = append(handlers, dag.HandlerOnFailure)
	case StatusCancel:
		handlers = append(handlers, dag.HandlerOnCancel)
	}
	handlers = append(handlers, dag.HandlerOnExit)
	for _, h := range handlers {
		if n := sc.handlers[h]; n != nil {
			log.Printf("%s started", n.data.Step.Name)

			n.mu.Lock()
			n.data.Step.OutputVariables = g.outputVariables
			n.mu.Unlock()

			if err := sc.runHandlerNode(ctx, n); err != nil {
				sc.setLastError(err)
			}
			if done != nil {
				done <- n
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

func (sc *Scheduler) setupNode(node *Node) error {
	if !sc.Dry {
		return node.setup(sc.LogDir, sc.ReqID)
	}
	return nil
}

func (sc *Scheduler) teardownNode(node *Node) error {
	if !sc.Dry {
		return node.teardown()
	}
	return nil
}

func (sc *Scheduler) execNode(ctx context.Context, n *Node) error {
	if !sc.Dry {
		return n.Execute(ctx)
	}
	return nil
}

// Signal sends a signal to the scheduler.
// for a node with repeat policy, it does not stop the node and
// wait to finish current run.
func (sc *Scheduler) Signal(
	// nolint
	g *ExecutionGraph, sig os.Signal, done chan bool, allowOverride bool,
) {
	if !sc.isCanceled() {
		sc.setCanceled()
	}
	for _, node := range g.Nodes() {
		// for a repetitive task, we'll wait for the job to finish
		// until time reaches max wait time
		if !node.data.Step.RepeatPolicy.Repeat {
			node.signal(sig, allowOverride)
		}
	}
	if done != nil {
		defer func() {
			done <- true
		}()
		for g.IsRunning() {
			time.Sleep(sc.pause)
		}
	}
}

// Cancel sends -1 signal to all nodes.
func (sc *Scheduler) Cancel(g *ExecutionGraph) {
	sc.setCanceled()
	for _, node := range g.Nodes() {
		node.cancel()
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
func (sc *Scheduler) HandlerNode(name dag.HandlerType) *Node {
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

func isReady(g *ExecutionGraph, node *Node) bool {
	ready := true
	for _, dep := range g.to[node.id] {
		n := g.node(dep)
		switch n.State().Status {
		case NodeStatusSuccess:
			continue
		case NodeStatusError:
			if !n.data.Step.ContinueOn.Failure {
				ready = false
				node.setStatus(NodeStatusCancel)
				node.SetError(errUpstreamFailed)
			}
		case NodeStatusSkipped:
			if !n.data.Step.ContinueOn.Skipped {
				ready = false
				node.setStatus(NodeStatusSkipped)
				node.SetError(errUpstreamSkipped)
			}
		case NodeStatusCancel:
			ready = false
			node.setStatus(NodeStatusCancel)
		case NodeStatusNone, NodeStatusRunning:
			ready = false
		default:
			ready = false
		}
	}
	return ready
}

func (sc *Scheduler) runHandlerNode(ctx context.Context, node *Node) error {
	defer func() {
		node.data.FinishedAt = time.Now()
	}()

	node.setStatus(NodeStatusRunning)

	if !sc.Dry {
		err := node.setup(sc.LogDir, sc.ReqID)
		if err != nil {
			node.setStatus(NodeStatusError)
			return nil
		}
		defer func() {
			_ = node.teardown()
		}()
		err = node.Execute(ctx)
		if err != nil {
			node.setStatus(NodeStatusError)
		} else {
			node.setStatus(NodeStatusSuccess)
		}
	} else {
		node.setStatus(NodeStatusSuccess)
	}

	return nil
}

func (sc *Scheduler) setup() (err error) {
	sc.pause = time.Millisecond * 100
	if !sc.Dry {
		if err = os.MkdirAll(sc.LogDir, 0755); err != nil {
			err = fmt.Errorf("failed to create log directory: %w", err)
			return err
		}
	}
	sc.handlers = map[dag.HandlerType]*Node{}
	if sc.OnExit != nil {
		sc.handlers[dag.HandlerOnExit] = &Node{data: NodeData{Step: *sc.OnExit}}
	}
	if sc.OnSuccess != nil {
		sc.handlers[dag.HandlerOnSuccess] =
			&Node{data: NodeData{Step: *sc.OnSuccess}}
	}
	if sc.OnFailure != nil {
		sc.handlers[dag.HandlerOnFailure] =
			&Node{data: NodeData{Step: *sc.OnFailure}}
	}
	if sc.OnCancel != nil {
		sc.handlers[dag.HandlerOnCancel] =
			&Node{data: NodeData{Step: *sc.OnCancel}}
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

var (
	errUpstreamFailed  = fmt.Errorf("upstream failed")
	errUpstreamSkipped = fmt.Errorf("upstream skipped")
)

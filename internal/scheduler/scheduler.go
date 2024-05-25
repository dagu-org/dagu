package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/constants"
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

var (
	errUpstreamFailed  = fmt.Errorf("upstream failed")
	errUpstreamSkipped = fmt.Errorf("upstream skipped")
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
	handlers  map[string]*Node
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
	RequestId     string
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
			if len(node.step.Preconditions) > 0 {
				log.Printf("checking pre conditions for \"%s\"", node.step.Name)
				if err := dag.EvalConditions(node.step.Preconditions); err != nil {
					log.Printf("%s", err.Error())
					node.setStatus(NodeStatusSkipped)
					node.SetError(err)
					continue NodesIteration
				}
			}
			wg.Add(1)

			log.Printf("start running: %s", node.step.Name)
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
							sc.lastError = execErr
						case node.step.RetryPolicy != nil && node.step.RetryPolicy.Limit > node.getRetryCount():
							// retry
							log.Printf("%s failed but scheduled for retry", node.step.Name)
							node.incRetryCount()
							log.Printf("sleep %s for retry", node.step.RetryPolicy.Interval)
							time.Sleep(node.step.RetryPolicy.Interval)
							node.setRetriedAt(time.Now())
							node.setStatus(NodeStatusNone)
						default:
							// finish the node
							node.setStatus(NodeStatusError)
							node.setErr(execErr)
							sc.lastError = execErr
						}
					}
					if node.State().Status != NodeStatusCancel {
						node.incDoneCount()
					}
					if node.step.RepeatPolicy.Repeat {
						if execErr == nil || node.step.ContinueOn.Failure {
							if !sc.isCanceled() {
								time.Sleep(node.step.RepeatPolicy.Interval)
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
					sc.lastError = err
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

	var handlers []string
	switch sc.Status(g) {
	case StatusSuccess:
		handlers = append(handlers, constants.OnSuccess)
	case StatusError:
		handlers = append(handlers, constants.OnFailure)
	case StatusCancel:
		handlers = append(handlers, constants.OnCancel)
	}
	handlers = append(handlers, constants.OnExit)
	for _, h := range handlers {
		if n := sc.handlers[h]; n != nil {
			log.Printf("%s started", n.step.Name)
			n.step.OutputVariables = g.outputVariables
			if err := sc.runHandlerNode(ctx, n); err != nil {
				sc.lastError = err
			}
			if done != nil {
				done <- n
			}
		}
	}
	return sc.lastError
}

func (sc *Scheduler) setupNode(node *Node) error {
	if !sc.Dry {
		return node.setup(sc.LogDir, sc.RequestId)
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
func (sc *Scheduler) Signal(g *ExecutionGraph, sig os.Signal, done chan bool, allowOverride bool) {
	if !sc.isCanceled() {
		sc.setCanceled()
	}
	for _, node := range g.Nodes() {
		if node.step.RepeatPolicy.Repeat {
			// for a repetitive task, we'll wait for the job to finish
			// until time reaches max wait time
		} else {
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
func (sc *Scheduler) HandlerNode(name string) *Node {
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
			if !n.step.ContinueOn.Failure {
				ready = false
				node.setStatus(NodeStatusCancel)
				node.SetError(errUpstreamFailed)
			}
		case NodeStatusSkipped:
			if !n.step.ContinueOn.Skipped {
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
		node.FinishedAt = time.Now()
	}()

	node.setStatus(NodeStatusRunning)

	if !sc.Dry {
		err := node.setup(sc.LogDir, sc.RequestId)
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
	if sc.LogDir == "" {
		sc.LogDir = config.Get().LogDir
	}
	if !sc.Dry {
		if err = os.MkdirAll(sc.LogDir, 0755); err != nil {
			return
		}
	}
	sc.handlers = map[string]*Node{}
	if sc.OnExit != nil {
		sc.handlers[constants.OnExit] = &Node{step: *sc.OnExit}
	}
	if sc.OnSuccess != nil {
		sc.handlers[constants.OnSuccess] = &Node{step: *sc.OnSuccess}
	}
	if sc.OnFailure != nil {
		sc.handlers[constants.OnFailure] = &Node{step: *sc.OnFailure}
	}
	if sc.OnCancel != nil {
		sc.handlers[constants.OnCancel] = &Node{step: *sc.OnCancel}
	}
	return
}

func (sc *Scheduler) setCanceled() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.canceled = 1
}

func (sc *Scheduler) runningCount(g *ExecutionGraph) int {
	count := 0
	for _, node := range g.Nodes() {
		if node.State().Status == NodeStatusRunning {
			count++
		}
	}
	return count
}

func (sc *Scheduler) isFinished(g *ExecutionGraph) bool {
	for _, node := range g.Nodes() {
		if node.State().Status == NodeStatusRunning || node.State().Status == NodeStatusNone {
			return false
		}
	}
	return true
}

func (sc *Scheduler) isSucceed(g *ExecutionGraph) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	for _, node := range g.Nodes() {
		if st := node.State().Status; st == NodeStatusSuccess || st == NodeStatusSkipped {
			continue
		}
		return false
	}
	return true
}

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
	"github.com/dagu-dev/dagu/internal/pb"
)

type SchedulerStatus int

const (
	SchedulerStatus_None SchedulerStatus = iota
	SchedulerStatus_Running
	SchedulerStatus_Error
	SchedulerStatus_Cancel
	SchedulerStatus_Success
	SchedulerStatus_Skipped_Unused
)

func (s SchedulerStatus) String() string {
	switch s {
	case SchedulerStatus_Running:
		return "running"
	case SchedulerStatus_Error:
		return "failed"
	case SchedulerStatus_Cancel:
		return "canceled"
	case SchedulerStatus_Success:
		return "finished"
	case SchedulerStatus_None:
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
	OnExit        *pb.Step
	OnSuccess     *pb.Step
	OnFailure     *pb.Step
	OnCancel      *pb.Step
	RequestId     string
}

// Schedule runs the graph of steps.
func (sc *Scheduler) Schedule(ctx context.Context, g *ExecutionGraph, done chan *Node) error {
	if err := sc.setup(); err != nil {
		return err
	}
	g.StartedAt = time.Now()

	defer func() {
		g.FinishedAt = time.Now()
	}()

	var wg = sync.WaitGroup{}

	for !sc.isFinished(g) {
		if sc.isCanceled() {
			break
		}
	NodesIteration:
		for _, node := range g.Nodes() {
			if node.GetStatus() != NodeStatus_None || !isReady(g, node) {
				continue NodesIteration
			}
			if sc.isCanceled() {
				break NodesIteration
			}
			if sc.MaxActiveRuns > 0 && sc.runningCount(g) >= sc.MaxActiveRuns {
				continue NodesIteration
			}
			// Check preconditions
			if len(node.Preconditions) > 0 {
				log.Printf("checking pre conditions for \"%s\"", node.Name)
				if err := dag.EvalConditions(node.Preconditions); err != nil {
					log.Printf("%s", err.Error())
					node.setStatus(NodeStatus_Skipped)
					node.Error = err
					continue NodesIteration
				}
			}
			wg.Add(1)

			log.Printf("start running: %s", node.Name)
			node.setStatus(NodeStatus_Running)
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
						status := node.GetStatus()
						switch {
						case status == NodeStatus_Success || status == NodeStatus_Cancel:
							// do nothing
						case sc.isCanceled():
							sc.lastError = execErr
						case node.RetryPolicy != nil && node.RetryPolicy.Limit > node.getRetryCount():
							// retry
							log.Printf("%s failed but scheduled for retry", node.Name)
							node.incRetryCount()
							log.Printf("sleep %s for retry", node.RetryPolicy.Interval)
							time.Sleep(node.RetryPolicy.Interval)
							node.setRetriedAt(time.Now())
							node.setStatus(NodeStatus_None)
						default:
							// finish the node
							node.setStatus(NodeStatus_Error)
							sc.lastError = execErr
						}
					}
					if node.GetStatus() != NodeStatus_Cancel {
						node.incDoneCount()
					}
					if node.RepeatPolicy.Repeat {
						if execErr == nil || node.ContinueOn.Failure {
							if !sc.isCanceled() {
								time.Sleep(node.RepeatPolicy.Interval)
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
				if node.GetStatus() == NodeStatus_Running {
					node.setStatus(NodeStatus_Success)
				}
				if err := sc.teardownNode(node); err != nil {
					sc.lastError = err
					node.setStatus(NodeStatus_Error)
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
	case SchedulerStatus_Success:
		handlers = append(handlers, constants.OnSuccess)
	case SchedulerStatus_Error:
		handlers = append(handlers, constants.OnFailure)
	case SchedulerStatus_Cancel:
		handlers = append(handlers, constants.OnCancel)
	}
	handlers = append(handlers, constants.OnExit)
	for _, h := range handlers {
		if n := sc.handlers[h]; n != nil {
			log.Printf("%s started", n.Name)
			n.OutputVariables = g.outputVariables
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
		if node.RepeatPolicy.Repeat {
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
		for sc.isRunning(g) {
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
func (sc *Scheduler) Status(g *ExecutionGraph) SchedulerStatus {
	if sc.isCanceled() && !sc.isSucceed(g) {
		return SchedulerStatus_Cancel
	}
	if g.StartedAt.IsZero() {
		return SchedulerStatus_None
	}
	if sc.isRunning(g) {
		return SchedulerStatus_Running
	}
	if sc.lastError != nil {
		return SchedulerStatus_Error
	}
	return SchedulerStatus_Success
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
		switch n.GetStatus() {
		case NodeStatus_Success:
			continue
		case NodeStatus_Error:
			if !n.ContinueOn.Failure {
				ready = false
				node.setStatus(NodeStatus_Cancel)
				node.Error = fmt.Errorf("upstream failed")
			}
		case NodeStatus_Skipped:
			if !n.ContinueOn.Skipped {
				ready = false
				node.setStatus(NodeStatus_Skipped)
				node.Error = fmt.Errorf("upstream skipped")
			}
		case NodeStatus_Cancel:
			ready = false
			node.setStatus(NodeStatus_Cancel)
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

	node.setStatus(NodeStatus_Running)

	if !sc.Dry {
		err := node.setup(sc.LogDir, sc.RequestId)
		if err != nil {
			node.setStatus(NodeStatus_Error)
			return nil
		}
		defer func() {
			_ = node.teardown()
		}()
		err = node.Execute(ctx)
		if err != nil {
			node.setStatus(NodeStatus_Error)
		} else {
			node.setStatus(NodeStatus_Success)
		}
	} else {
		node.setStatus(NodeStatus_Success)
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
		onExit, _ := pb.ToDagStep(sc.OnExit)
		sc.handlers[constants.OnExit] = &Node{Step: onExit}
	}
	if sc.OnSuccess != nil {
		onSuccess, _ := pb.ToDagStep(sc.OnSuccess)
		sc.handlers[constants.OnSuccess] = &Node{Step: onSuccess}
	}
	if sc.OnFailure != nil {
		onFailure, _ := pb.ToDagStep(sc.OnFailure)
		sc.handlers[constants.OnFailure] = &Node{Step: onFailure}
	}
	if sc.OnCancel != nil {
		onCancel, _ := pb.ToDagStep(sc.OnCancel)
		sc.handlers[constants.OnCancel] = &Node{Step: onCancel}
	}
	return
}

func handleError(node *Node) {
	status := node.GetStatus()
	if status != NodeStatus_Cancel && status != NodeStatus_Success {
		if node.RetryPolicy != nil && node.RetryPolicy.Limit > node.getRetryCount() {
			log.Printf("%s failed but scheduled for retry", node.Name)
			node.incRetryCount()
			log.Printf("sleep %s for retry", node.RetryPolicy.Interval)
			time.Sleep(node.RetryPolicy.Interval)
			node.setRetriedAt(time.Now())
			node.setStatus(NodeStatus_None)
		} else {
			node.setStatus(NodeStatus_Error)
		}
	}
}

func (sc *Scheduler) setCanceled() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.canceled = 1
}

func (sc *Scheduler) isRunning(g *ExecutionGraph) bool {
	for _, node := range g.Nodes() {
		if node.GetStatus() == NodeStatus_Running {
			return true
		}
	}
	return false
}

func (sc *Scheduler) runningCount(g *ExecutionGraph) int {
	count := 0
	for _, node := range g.Nodes() {
		switch node.GetStatus() {
		case NodeStatus_Running:
			count++
		}
	}
	return count
}

func (sc *Scheduler) isFinished(g *ExecutionGraph) bool {
	for _, node := range g.Nodes() {
		switch node.GetStatus() {
		case NodeStatus_Running, NodeStatus_None:
			return false
		}
	}
	return true
}

func (sc *Scheduler) isSucceed(g *ExecutionGraph) bool {
	for _, node := range g.Nodes() {
		if st := node.GetStatus(); st == NodeStatus_Success || st == NodeStatus_Skipped {
			continue
		}
		return false
	}
	return true
}

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
		if sc.IsCanceled() {
			break
		}
		for _, node := range g.Nodes() {
			if node.ReadStatus() != NodeStatus_None {
				continue
			}
			if !isReady(g, node) {
				continue
			}
			if sc.IsCanceled() {
				break
			}
			if sc.MaxActiveRuns > 0 &&
				sc.runningCount(g) >= sc.MaxActiveRuns {
				continue
			}
			if len(node.Preconditions) > 0 {
				log.Printf("checking pre conditions for \"%s\"", node.Name)
				if err := dag.EvalConditions(node.Preconditions); err != nil {
					log.Printf("%s", err.Error())
					node.updateStatus(NodeStatus_Skipped)
					node.Error = err
					continue
				}
			}
			wg.Add(1)

			log.Printf("start running: %s", node.Name)
			node.updateStatus(NodeStatus_Running)
			go func(node *Node) {
				defer func() {
					node.FinishedAt = time.Now()
					wg.Done()
				}()

				setup := true
				if !sc.Dry {
					if err := node.setup(sc.LogDir, sc.RequestId); err != nil {
						setup = false
						node.Error = err
						sc.lastError = err
						node.updateStatus(NodeStatus_Error)
					}
					defer func() {
						_ = node.teardown()
					}()
				}

				for setup && !sc.IsCanceled() {
					var err error = nil
					if !sc.Dry {
						err = node.Execute(ctx)
					}
					if err != nil {
						if sc.IsCanceled() {
							if node.ReadStatus() != NodeStatus_Cancel {
								sc.lastError = err
							}
						} else {
							handleError(node)
						}
						switch node.ReadStatus() {
						case NodeStatus_None:
							// nothing to do
						case NodeStatus_Error:
							sc.lastError = err
						}
					}
					if node.ReadStatus() != NodeStatus_Cancel {
						node.incDoneCount()
					}
					if node.RepeatPolicy.Repeat {
						if err == nil || node.ContinueOn.Failure {
							if !sc.IsCanceled() {
								time.Sleep(node.RepeatPolicy.Interval)
								continue
							}
						}
					}
					if err != nil {
						if done != nil {
							done <- node
						}
						return
					}
					break
				}
				if node.ReadStatus() == NodeStatus_Running {
					node.updateStatus(NodeStatus_Success)
				}
				if err := node.teardown(); err != nil {
					sc.lastError = err
					node.updateStatus(NodeStatus_Error)
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

	handlers := []string{}
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
			err := sc.runHandlerNode(ctx, n)
			if err != nil {
				sc.lastError = err
			}
			if done != nil {
				done <- n
			}
		}
	}
	return sc.lastError
}

// Signal sends a signal to the scheduler.
// for a node with repeat policy, it does not stop the node and
// wait to finish current run.
func (sc *Scheduler) Signal(g *ExecutionGraph, sig os.Signal, done chan bool, allowOverride bool) {
	if !sc.IsCanceled() {
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
	if sc.IsCanceled() && !sc.checkStatus(g, []NodeStatus{
		NodeStatus_Success, NodeStatus_Skipped,
	}) {
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

// IsCanceled returns true if the scheduler is canceled.
func (sc *Scheduler) IsCanceled() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	ret := sc.canceled == 1
	return ret
}

func isReady(g *ExecutionGraph, node *Node) (ready bool) {
	ready = true
	for _, dep := range g.to[node.id] {
		n := g.node(dep)
		switch n.ReadStatus() {
		case NodeStatus_Success:
			continue
		case NodeStatus_Error:
			if !n.ContinueOn.Failure {
				ready = false
				node.updateStatus(NodeStatus_Cancel)
				node.Error = fmt.Errorf("upstream failed")
			}
		case NodeStatus_Skipped:
			if !n.ContinueOn.Skipped {
				ready = false
				node.updateStatus(NodeStatus_Skipped)
				node.Error = fmt.Errorf("upstream skipped")
			}
		case NodeStatus_Cancel:
			ready = false
			node.updateStatus(NodeStatus_Cancel)
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

	node.updateStatus(NodeStatus_Running)

	if !sc.Dry {
		err := node.setup(sc.LogDir, sc.RequestId)
		if err != nil {
			node.updateStatus(NodeStatus_Error)
			return nil
		}
		defer func() {
			_ = node.teardown()
		}()
		err = node.Execute(ctx)
		if err != nil {
			node.updateStatus(NodeStatus_Error)
		} else {
			node.updateStatus(NodeStatus_Success)
		}
	} else {
		node.updateStatus(NodeStatus_Success)
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
	status := node.ReadStatus()
	if status != NodeStatus_Cancel && status != NodeStatus_Success {
		if node.RetryPolicy != nil && node.RetryPolicy.Limit > node.ReadRetryCount() {
			log.Printf("%s failed but scheduled for retry", node.Name)
			node.incRetryCount()
			log.Printf("sleep %s for retry", node.RetryPolicy.Interval)
			time.Sleep(node.RetryPolicy.Interval)
			node.SetRetriedAt(time.Now())
			node.updateStatus(NodeStatus_None)
		} else {
			node.updateStatus(NodeStatus_Error)
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
		switch node.ReadStatus() {
		case NodeStatus_Running:
			return true
		}
	}
	return false
}

func (sc *Scheduler) runningCount(g *ExecutionGraph) (count int) {
	count = 0
	for _, node := range g.Nodes() {
		switch node.ReadStatus() {
		case NodeStatus_Running:
			count++
		}
	}
	return count
}

func (sc *Scheduler) isFinished(g *ExecutionGraph) bool {
	for _, node := range g.Nodes() {
		switch node.ReadStatus() {
		case NodeStatus_Running, NodeStatus_None:
			return false
		}
	}
	return true
}

func (sc *Scheduler) checkStatus(g *ExecutionGraph, in []NodeStatus) bool {
	for _, node := range g.Nodes() {
		s := node.ReadStatus()
		var f = false
		for i := range in {
			f = s == in[i]
			if f {
				break
			}
		}
		if !f {
			return false
		}
	}
	return true
}

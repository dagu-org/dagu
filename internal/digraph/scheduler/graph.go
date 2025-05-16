package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
)

// ExecutionGraph represents a graph of steps.
type ExecutionGraph struct {
	startedAt  time.Time
	finishedAt time.Time
	nodeByID   map[int]*Node
	nodes      []*Node
	From       map[int][]int
	To         map[int][]int
	lock       sync.RWMutex
}

// NewExecutionGraph creates a new execution graph with the given steps.
func NewExecutionGraph(steps ...digraph.Step) (*ExecutionGraph, error) {
	graph := &ExecutionGraph{
		nodeByID: make(map[int]*Node),
		From:     make(map[int][]int),
		To:       make(map[int][]int),
		nodes:    []*Node{},
	}
	for _, step := range steps {
		node := &Node{Data: newSafeData(NodeData{Step: step})}
		node.Init()
		graph.nodeByID[node.id] = node
		graph.nodes = append(graph.nodes, node)
	}
	if err := graph.setup(); err != nil {
		return nil, err
	}
	return graph, nil
}

// CreateRetryExecutionGraph creates a new execution graph for retry with
// given nodes.
func CreateRetryExecutionGraph(ctx context.Context, dag *digraph.DAG, nodes ...*Node) (*ExecutionGraph, error) {
	graph := &ExecutionGraph{
		nodeByID: make(map[int]*Node),
		From:     make(map[int][]int),
		To:       make(map[int][]int),
		nodes:    []*Node{},
	}
	steps := make(map[string]digraph.Step)
	for _, step := range dag.Steps {
		steps[step.Name] = step
	}
	for _, node := range nodes {
		node.Init()
		graph.nodeByID[node.id] = node
		graph.nodes = append(graph.nodes, node)
	}
	if err := graph.setup(); err != nil {
		return nil, err
	}
	if err := graph.setupRetry(ctx, steps); err != nil {
		return nil, err
	}
	return graph, nil
}

// Duration returns the duration of the execution.
func (g *ExecutionGraph) Duration() time.Duration {
	g.lock.RLock()
	defer g.lock.RUnlock()
	if g.finishedAt.IsZero() {
		return time.Since(g.startedAt)
	}
	return g.finishedAt.Sub(g.startedAt)
}

func (g *ExecutionGraph) IsStarted() bool {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return !g.startedAt.IsZero()
}

func (g *ExecutionGraph) IsFinished() bool {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return !g.finishedAt.IsZero()
}

func (g *ExecutionGraph) StartAt() time.Time {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.startedAt
}

func (g *ExecutionGraph) IsRunning() bool {
	g.lock.RLock()
	defer g.lock.RUnlock()
	for _, node := range g.nodes {
		if node.State().Status == NodeStatusRunning {
			return true
		}
	}
	return false
}

func (g *ExecutionGraph) FinishAt() time.Time {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.finishedAt
}

func (g *ExecutionGraph) Finish() {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.finishedAt = time.Now()
}

func (g *ExecutionGraph) Start() {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.startedAt = time.Now()
}

func (g *ExecutionGraph) NodeData() []NodeData {
	g.lock.Lock()
	defer g.lock.Unlock()

	var ret []NodeData
	for _, node := range g.nodes {
		node.mu.Lock()
		ret = append(ret, node.NodeData())
		node.mu.Unlock()
	}

	return ret
}

func (g *ExecutionGraph) NodeByName(name string) *Node {
	for _, node := range g.nodes {
		if node.Name() == name {
			return node
		}
	}
	return nil
}

func (g *ExecutionGraph) setupRetry(ctx context.Context, steps map[string]digraph.Step) error {
	dict := map[int]NodeStatus{}
	retry := map[int]bool{}
	for _, node := range g.nodes {
		dict[node.id] = node.Status()
		retry[node.id] = false
	}
	var frontier []int
	for _, node := range g.nodes {
		if len(node.Step().Depends) == 0 {
			frontier = append(frontier, node.id)
		}
	}
	for len(frontier) > 0 {
		var next []int
		for _, u := range frontier {
			if retry[u] || dict[u] == NodeStatusError ||
				dict[u] == NodeStatusCancel {
				logger.Debug(ctx, "Clearing node state", "step", g.nodeByID[u].Name())
				step, ok := steps[g.nodeByID[u].Name()]
				if !ok {
					// This should never happen, but just in case
					return fmt.Errorf("%w: %s", errStepNotFound, g.nodeByID[u].Name())
				}
				g.nodeByID[u].ClearState(step)
				retry[u] = true
			}
			for _, v := range g.From[u] {
				if retry[u] {
					retry[v] = true
				}
				next = append(next, v)
			}
		}
		frontier = next
	}
	return nil
}

func (g *ExecutionGraph) setup() error {
	for _, node := range g.nodes {
		for _, dep := range node.Step().Depends {
			depStep, err := g.findStep(dep)
			if err != nil {
				return err
			}
			g.addEdge(depStep, node)
		}
	}

	if g.hasCycle() {
		return errCycleDetected
	}

	return nil
}

func (g *ExecutionGraph) hasCycle() bool {
	var inDegrees = make(map[int]int)
	for node, depends := range g.To {
		inDegrees[node] = len(depends)
	}

	var q []int
	for _, node := range g.nodes {
		if inDegrees[node.id] != 0 {
			continue
		}
		q = append(q, node.id)
	}

	for len(q) > 0 {
		var f = q[0]
		q = q[1:]

		var tos = g.From[f]
		for _, to := range tos {
			inDegrees[to]--
			if inDegrees[to] == 0 {
				q = append(q, to)
			}
		}
	}

	for _, degree := range inDegrees {
		if degree > 0 {
			return true
		}
	}

	return false
}

func (g *ExecutionGraph) addEdge(from, to *Node) {
	g.From[from.id] = append(g.From[from.id], to.id)
	g.To[to.id] = append(g.To[to.id], from.id)
}

func (g *ExecutionGraph) findStep(name string) (*Node, error) {
	for _, n := range g.nodeByID {
		if n.Name() == name {
			return n, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", errStepNotFound, name)
}

var (
	errCycleDetected = errors.New("cycle detected")
	errStepNotFound  = errors.New("step not found")
)

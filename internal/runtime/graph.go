package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core"
)

// ExecutionGraph represents a graph of steps.
type ExecutionGraph struct {
	startedAt  time.Time
	finishedAt time.Time
	nodeByID   map[int]*Node
	nodeByName map[string]*Node // faster lookup by step name
	nodes      []*Node
	From       map[int][]int
	To         map[int][]int
	lock       sync.RWMutex
}

// newGraph allocates an empty execution graph. Construction helpers populate nodes then call buildEdges.
func newGraph() *ExecutionGraph {
	return &ExecutionGraph{
		nodeByID:   make(map[int]*Node),
		nodeByName: make(map[string]*Node),
		From:       make(map[int][]int),
		To:         make(map[int][]int),
		nodes:      []*Node{},
		startedAt:  time.Now(),
	}
}

// NewExecutionGraph creates a new execution graph with the given steps.
func NewExecutionGraph(steps ...core.Step) (*ExecutionGraph, error) {
	graph := newGraph()
	for _, step := range steps {
		node := &Node{Data: newSafeData(NodeData{Step: step})}
		node.Init()
		graph.nodeByID[node.id] = node
		graph.nodeByName[node.Name()] = node
		graph.nodes = append(graph.nodes, node)
	}
	if err := graph.buildEdges(); err != nil {
		return nil, err
	}
	return graph, nil
}

// CreateRetryExecutionGraph creates a new execution graph for retry with
// given nodes.
func CreateRetryExecutionGraph(ctx context.Context, dag *core.DAG, nodes ...*Node) (*ExecutionGraph, error) {
	graph := newGraph()
	steps := stepsByName(dag)
	for _, node := range nodes {
		node.Init()
		graph.nodeByID[node.id] = node
		graph.nodeByName[node.Name()] = node
		graph.nodes = append(graph.nodes, node)
	}
	if err := graph.buildEdges(); err != nil {
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
		s := node.State().Status
		if s == core.NodeRunning {
			return true
		}
		if s == core.NodeNotStarted && g.finishedAt.IsZero() {
			// If the node is not started and the graph is not finished,
			// it means the node is still pending to be executed.
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

func (g *ExecutionGraph) NodeData() []NodeData {
	g.lock.RLock() // graph itself is not mutated
	defer g.lock.RUnlock()
	var ret []NodeData
	for _, node := range g.nodes {
		node.mu.Lock()
		ret = append(ret, node.NodeData())
		node.mu.Unlock()
	}
	return ret
}

func (g *ExecutionGraph) NodeByName(name string) *Node {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.nodeByName[name]
}

func (g *ExecutionGraph) setupRetry(ctx context.Context, steps map[string]core.Step) error {
	dict := map[int]core.NodeStatus{}
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
			if retry[u] || dict[u] == core.NodeFailed ||
				dict[u] == core.NodeAborted {
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

// buildEdges populates dependency edges and validates acyclicity.
func (g *ExecutionGraph) buildEdges() error {
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
	if n, ok := g.nodeByName[name]; ok {
		return n, nil
	}
	return nil, fmt.Errorf("%w: %s", errStepNotFound, name)
}

func (g *ExecutionGraph) isFinished() bool {
	for _, node := range g.nodes {
		if node.State().Status == core.NodeRunning ||
			node.State().Status == core.NodeNotStarted {
			return false
		}
	}
	return true
}

func (g *ExecutionGraph) runningCount() int {
	count := 0
	for _, node := range g.nodes {
		if node.State().Status == core.NodeRunning {
			count++
		}
	}
	return count
}

// CreateStepRetryGraph creates a new execution graph for retrying a specific step.
// Only the specified step will be reset for re-execution, leaving all downstream steps untouched.
func CreateStepRetryGraph(dag *core.DAG, nodes []*Node, stepName string) (*ExecutionGraph, error) {
	graph := newGraph()
	steps := stepsByName(dag)
	for _, node := range nodes {
		node.Init()
		graph.nodeByID[node.id] = node
		graph.nodeByName[node.Name()] = node
		graph.nodes = append(graph.nodes, node)
	}
	if err := graph.buildEdges(); err != nil {
		return nil, err
	}
	targetNode := graph.NodeByName(stepName)
	if targetNode == nil {
		return nil, fmt.Errorf("%w: %s", errStepNotFound, stepName)
	}
	step, ok := steps[targetNode.Name()]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errStepNotFound, targetNode.Name())
	}
	targetNode.ClearState(step)
	targetNode.retryPolicy = RetryPolicy{} // force a fresh retry without prior policy
	return graph, nil
}

// stepsByName creates a name->step map for convenience.
func stepsByName(dag *core.DAG) map[string]core.Step {
	m := make(map[string]core.Step, len(dag.Steps))
	for _, s := range dag.Steps {
		m[s.Name] = s
	}
	return m
}

var (
	errCycleDetected = errors.New("cycle detected")
	errStepNotFound  = errors.New("step not found")
)

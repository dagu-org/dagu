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

var (
	ErrCycleDetected = errors.New("cycle detected")
	ErrStepNotFound  = errors.New("step not found")
)

// ExecutionPlan represents a plan of execution for a set of steps.
// It encapsulates the graph structure and ensures thread-safe access.
type ExecutionPlan struct {
	startedAt  time.Time
	finishedAt time.Time

	// Graph structure (immutable after construction)
	nodes        []*Node
	nodeByID     map[int]*Node
	nodeByName   map[string]*Node
	dependencies map[int][]int // node ID -> list of dependency node IDs (upstream)
	dependents   map[int][]int // node ID -> list of dependent node IDs (downstream)

	mu sync.RWMutex
}

// NewExecutionPlan creates a new execution plan from the given steps.
// It builds the graph, validates it (checking for cycles), and returns the plan.
func NewExecutionPlan(steps ...core.Step) (*ExecutionPlan, error) {
	p := &ExecutionPlan{
		nodeByID:     make(map[int]*Node),
		nodeByName:   make(map[string]*Node),
		dependencies: make(map[int][]int),
		dependents:   make(map[int][]int),
		nodes:        make([]*Node, 0, len(steps)),
		startedAt:    time.Now(),
	}

	// Initialize nodes
	for _, step := range steps {
		node := &Node{Data: newSafeData(NodeData{Step: step})}
		node.Init()
		p.addNode(node)
	}

	// Build edges
	if err := p.buildEdges(); err != nil {
		return nil, err
	}

	return p, nil
}

// CreateRetryExecutionPlan creates a new execution plan for retrying specific nodes.
func CreateRetryExecutionPlan(ctx context.Context, dag *core.DAG, nodes ...*Node) (*ExecutionPlan, error) {
	p := &ExecutionPlan{
		nodeByID:     make(map[int]*Node),
		nodeByName:   make(map[string]*Node),
		dependencies: make(map[int][]int),
		dependents:   make(map[int][]int),
		nodes:        make([]*Node, 0, len(nodes)),
		startedAt:    time.Now(),
	}

	steps := stepsByName(dag)

	// Initialize nodes
	for _, node := range nodes {
		node.Init()
		p.addNode(node)
	}

	// Build edges
	if err := p.buildEdges(); err != nil {
		return nil, err
	}

	// Setup retry state
	if err := p.setupRetry(ctx, steps); err != nil {
		return nil, err
	}

	return p, nil
}

// CreateStepRetryPlan creates a new execution plan for retrying a specific step.
func CreateStepRetryPlan(dag *core.DAG, nodes []*Node, stepName string) (*ExecutionPlan, error) {
	p := &ExecutionPlan{
		nodeByID:     make(map[int]*Node),
		nodeByName:   make(map[string]*Node),
		dependencies: make(map[int][]int),
		dependents:   make(map[int][]int),
		nodes:        make([]*Node, 0, len(nodes)),
		startedAt:    time.Now(),
	}

	steps := stepsByName(dag)

	for _, node := range nodes {
		node.Init()
		p.addNode(node)
	}

	if err := p.buildEdges(); err != nil {
		return nil, err
	}

	targetNode := p.GetNodeByName(stepName)
	if targetNode == nil {
		return nil, fmt.Errorf("%w: %s", ErrStepNotFound, stepName)
	}

	step, ok := steps[targetNode.Name()]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrStepNotFound, targetNode.Name())
	}

	targetNode.ClearState(step)
	targetNode.retryPolicy = RetryPolicy{} // force a fresh retry without prior policy

	return p, nil
}

// addNode adds a node to the plan structures.
func (p *ExecutionPlan) addNode(node *Node) {
	p.nodeByID[node.id] = node
	p.nodeByName[node.Name()] = node
	p.nodes = append(p.nodes, node)
}

// buildEdges populates dependency edges and validates acyclicity.
func (p *ExecutionPlan) buildEdges() error {
	for _, node := range p.nodes {
		for _, depName := range node.Step().Depends {
			depNode, ok := p.nodeByName[depName]
			if !ok {
				return fmt.Errorf("%w: %s", ErrStepNotFound, depName)
			}
			p.addEdge(depNode, node)
		}
	}

	if p.hasCycle() {
		return ErrCycleDetected
	}
	return nil
}

// addEdge adds a directed edge from 'from' to 'to'.
func (p *ExecutionPlan) addEdge(from, to *Node) {
	p.dependents[from.id] = append(p.dependents[from.id], to.id)
	p.dependencies[to.id] = append(p.dependencies[to.id], from.id)
}

// hasCycle checks for cycles in the graph using Kahn's algorithm.
func (p *ExecutionPlan) hasCycle() bool {
	inDegrees := make(map[int]int)
	for id, deps := range p.dependencies {
		inDegrees[id] = len(deps)
	}

	var queue []int
	for _, node := range p.nodes {
		if inDegrees[node.id] == 0 {
			queue = append(queue, node.id)
		}
	}

	processedCount := 0
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		processedCount++

		for _, v := range p.dependents[u] {
			inDegrees[v]--
			if inDegrees[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	return processedCount != len(p.nodes)
}

// setupRetry resets the state of failed/aborted nodes and their dependents.
func (p *ExecutionPlan) setupRetry(ctx context.Context, steps map[string]core.Step) error {
	// Identify nodes that need to be retried (failed or aborted)
	toRetry := make(map[int]bool)
	nodeStatus := make(map[int]core.NodeStatus)

	for _, node := range p.nodes {
		nodeStatus[node.id] = node.Status()
		toRetry[node.id] = false
	}

	var frontier []int
	for _, node := range p.nodes {
		if len(p.dependencies[node.id]) == 0 {
			frontier = append(frontier, node.id)
		}
	}

	for len(frontier) > 0 {
		var next []int
		for _, u := range frontier {
			shouldRetry := toRetry[u] ||
				nodeStatus[u] == core.NodeFailed ||
				nodeStatus[u] == core.NodeAborted

			if shouldRetry {
				node := p.nodeByID[u]
				logger.Debug(ctx, "Clearing node state", "step", node.Name())
				step, ok := steps[node.Name()]
				if !ok {
					return fmt.Errorf("%w: %s", ErrStepNotFound, node.Name())
				}
				node.ClearState(step)
				toRetry[u] = true
			}

			for _, v := range p.dependents[u] {
				if toRetry[u] {
					toRetry[v] = true
				}
				next = append(next, v)
			}
		}
		frontier = next
	}

	return nil
}

// --- Accessors ---

// Nodes returns a slice of all nodes in the plan.
func (p *ExecutionPlan) Nodes() []*Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// Return a copy to prevent modification of the underlying slice
	nodes := make([]*Node, len(p.nodes))
	copy(nodes, p.nodes)
	return nodes
}

// GetNode returns the node with the given ID.
func (p *ExecutionPlan) GetNode(id int) *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodeByID[id]
}

// GetNodeByName returns the node with the given name.
func (p *ExecutionPlan) GetNodeByName(name string) *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodeByName[name]
}

// Dependencies returns the IDs of the nodes that the given node depends on.
func (p *ExecutionPlan) Dependencies(nodeID int) []int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	deps := p.dependencies[nodeID]
	result := make([]int, len(deps))
	copy(result, deps)
	return result
}

// Dependents returns the IDs of the nodes that depend on the given node.
func (p *ExecutionPlan) Dependents(nodeID int) []int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	deps := p.dependents[nodeID]
	result := make([]int, len(deps))
	copy(result, deps)
	return result
}

// --- Status & Time ---

func (p *ExecutionPlan) StartAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.startedAt
}

func (p *ExecutionPlan) FinishAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.finishedAt
}

func (p *ExecutionPlan) Duration() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.finishedAt.IsZero() {
		return time.Since(p.startedAt)
	}
	return p.finishedAt.Sub(p.startedAt)
}

func (p *ExecutionPlan) IsStarted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.startedAt.IsZero()
}

func (p *ExecutionPlan) IsFinished() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.finishedAt.IsZero()
}

func (p *ExecutionPlan) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finishedAt = time.Now()
}

// IsRunning checks if any node is currently running or pending.
func (p *ExecutionPlan) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, node := range p.nodes {
		s := node.State().Status
		if s == core.NodeRunning {
			return true
		}
		if s == core.NodeNotStarted && p.finishedAt.IsZero() {
			return true
		}
	}
	return false
}

// CheckFinished checks if all nodes have completed (successfully or otherwise).
func (p *ExecutionPlan) CheckFinished() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, node := range p.nodes {
		if node.State().Status == core.NodeRunning ||
			node.State().Status == core.NodeNotStarted {
			return false
		}
	}
	return true
}

// NodeData returns a snapshot of data for all nodes.
func (p *ExecutionPlan) NodeData() []NodeData {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var ret []NodeData
	for _, node := range p.nodes {
		// Node's internal lock is handled by NodeData()
		ret = append(ret, node.NodeData())
	}
	return ret
}

// Helper
func stepsByName(dag *core.DAG) map[string]core.Step {
	m := make(map[string]core.Step, len(dag.Steps))
	for _, s := range dag.Steps {
		m[s.Name] = s
	}
	return m
}

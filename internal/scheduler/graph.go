package scheduler

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/utils"
)

// ExecutionGraph represents a graph of steps.
type ExecutionGraph struct {
	StartedAt       time.Time
	FinishedAt      time.Time
	outputVariables *utils.SyncMap
	dict            map[int]*Node
	nodes           []*Node
	from            map[int][]int
	to              map[int][]int
}

// NewExecutionGraph creates a new execution graph with the given steps.
func NewExecutionGraph(steps ...*dag.Step) (*ExecutionGraph, error) {
	graph := &ExecutionGraph{
		outputVariables: &utils.SyncMap{},
		dict:            make(map[int]*Node),
		from:            make(map[int][]int),
		to:              make(map[int][]int),
		nodes:           []*Node{},
	}
	for _, step := range steps {
		step.OutputVariables = graph.outputVariables
		node := &Node{Step: step}
		node.init()
		graph.dict[node.id] = node
		graph.nodes = append(graph.nodes, node)
	}
	if err := graph.setup(); err != nil {
		return nil, err
	}
	return graph, nil
}

// NewExecutionGraphForRetry creates a new execution graph for retry with given nodes.
func NewExecutionGraphForRetry(nodes ...*Node) (*ExecutionGraph, error) {
	graph := &ExecutionGraph{
		outputVariables: &utils.SyncMap{},
		dict:            make(map[int]*Node),
		from:            make(map[int][]int),
		to:              make(map[int][]int),
		nodes:           []*Node{},
	}
	for _, node := range nodes {
		if node.OutputVariables != nil {
			node.OutputVariables.Range(func(key, value interface{}) bool {
				k := key.(string)
				v := value.(string)
				graph.outputVariables.Store(key, value)
				err := os.Setenv(k, v[len(key.(string))+1:])
				if err != nil {
					log.Printf("set env error : %s", err.Error())
				}
				return true
			})
		}
		node.OutputVariables = graph.outputVariables
		node.init()
		graph.dict[node.id] = node
		graph.nodes = append(graph.nodes, node)
	}
	if err := graph.setup(); err != nil {
		return nil, err
	}
	if err := graph.setupRetry(); err != nil {
		return nil, err
	}
	return graph, nil
}

// Duration returns the duration of the execution.
func (g *ExecutionGraph) Duration() time.Duration {
	if g.FinishedAt.IsZero() {
		return time.Since(g.StartedAt)
	}
	return g.FinishedAt.Sub(g.StartedAt)
}

// Nodes returns the nodes of the execution graph.
func (g *ExecutionGraph) Nodes() []*Node {
	return g.nodes
}

func (g *ExecutionGraph) node(id int) *Node {
	return g.dict[id]
}

func (g *ExecutionGraph) setupRetry() error {
	dict := map[int]NodeStatus{}
	retry := map[int]bool{}
	for _, node := range g.nodes {
		dict[node.id] = node.Status
		retry[node.id] = false
	}
	frontier := []int{}
	for _, node := range g.nodes {
		if len(node.Depends) == 0 {
			frontier = append(frontier, node.id)
		}
	}
	for len(frontier) > 0 {
		next := []int{}
		for _, u := range frontier {
			if retry[u] || dict[u] == NodeStatus_Error || dict[u] == NodeStatus_Cancel {
				log.Printf("clear node state: %s", g.dict[u].Name)
				g.dict[u].clearState()
				retry[u] = true
			}
			for _, v := range g.from[u] {
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
		for _, dep := range node.Depends {
			depStep, err := g.findStep(dep)
			if err != nil {
				return err
			}
			g.addEdge(depStep, node)
		}
	}

	if g.hasCycle() {
		return fmt.Errorf("cycle detected")
	}

	return nil
}

func (g *ExecutionGraph) hasCycle() bool {
	var inDegrees = make(map[int]int)
	for node, depends := range g.to {
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

		var tos = g.from[f]
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
	g.from[from.id] = append(g.from[from.id], to.id)
	g.to[to.id] = append(g.to[to.id], from.id)
}

func (g *ExecutionGraph) findStep(name string) (*Node, error) {
	for _, n := range g.dict {
		if n.Name == name {
			return n, nil
		}
	}
	return nil, fmt.Errorf("step not found: %s", name)
}

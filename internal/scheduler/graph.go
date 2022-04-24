package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/yohamta/jobctl/internal/config"
)

type ExecutionGraph struct {
	dict                  map[int]*Node
	nodes                 []*Node
	from                  map[int][]int
	to                    map[int][]int
	StartedAt, FinishedAt time.Time
}

func NewExecutionGraph(steps ...*config.Step) (*ExecutionGraph, error) {
	graph := &ExecutionGraph{
		dict:  make(map[int]*Node),
		from:  make(map[int][]int),
		to:    make(map[int][]int),
		nodes: []*Node{},
	}
	for _, step := range steps {
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

func RetryExecutionGraph(nodes ...*Node) (*ExecutionGraph, error) {
	graph := &ExecutionGraph{
		dict:  make(map[int]*Node),
		from:  make(map[int][]int),
		to:    make(map[int][]int),
		nodes: []*Node{},
	}
	for _, node := range nodes {
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

func (g *ExecutionGraph) Duration() time.Duration {
	if g.FinishedAt.IsZero() {
		return time.Since(g.StartedAt)
	}
	return g.FinishedAt.Sub(g.StartedAt)
}

func (g *ExecutionGraph) Nodes() []*Node {
	return g.nodes
}

func (g *ExecutionGraph) From(from int) []int {
	return g.from[from]
}

func (g *ExecutionGraph) To(to int) []int {
	return g.to[to]
}

func (g *ExecutionGraph) Node(id int) *Node {
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
			if retry[u] == true || dict[u] == NodeStatus_Error || dict[u] == NodeStatus_Cancel {
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
			dep_step, err := g.findStep(dep)
			if err != nil {
				return err
			}
			err = g.addEdge(dep_step, node)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *ExecutionGraph) addEdge(from, to *Node) error {
	g.from[from.id] = append(g.from[from.id], to.id)
	g.to[to.id] = append(g.to[to.id], from.id)
	return g.cycleDfs(to.id, make(map[int]bool))
}

func (g *ExecutionGraph) cycleDfs(t int, visited map[int]bool) error {
	if visited[t] {
		return fmt.Errorf("cycle detected")
	}
	visited[t] = true
	for _, next := range g.from[t] {
		err := g.cycleDfs(next, visited)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *ExecutionGraph) findStep(name string) (*Node, error) {
	for _, n := range g.dict {
		if n.Name == name {
			return n, nil
		}
	}
	return nil, fmt.Errorf("step not found: %s", name)
}

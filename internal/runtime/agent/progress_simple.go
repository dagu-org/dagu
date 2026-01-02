package agent

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SimpleProgressDisplay provides a minimal inline progress display.
type SimpleProgressDisplay struct {
	dag      *core.DAG
	dagRunID string

	mu            sync.Mutex
	total         int
	completed     int
	completedNodes map[string]bool // track which nodes are already counted
	status        core.Status
	spinnerIndex  int

	stopCh chan struct{}
	done   chan struct{}
}

// NewSimpleProgressDisplay creates a new simple progress display.
func NewSimpleProgressDisplay(dag *core.DAG) *SimpleProgressDisplay {
	total := 0
	if dag != nil {
		total = len(dag.Steps)
	}
	return &SimpleProgressDisplay{
		dag:            dag,
		total:          total,
		completedNodes: make(map[string]bool),
		stopCh:         make(chan struct{}),
		done:           make(chan struct{}),
	}
}

// Start begins the progress display.
func (p *SimpleProgressDisplay) Start() {
	go p.run()
}

// Stop stops the progress display.
func (p *SimpleProgressDisplay) Stop() {
	close(p.stopCh)
	<-p.done
}

// UpdateNode updates the progress for a specific node.
func (p *SimpleProgressDisplay) UpdateNode(node *execution.Node) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Only count completed nodes once
	if node.Status == core.NodeSucceeded || node.Status == core.NodeFailed ||
		node.Status == core.NodeSkipped || node.Status == core.NodeAborted {
		if !p.completedNodes[node.Step.Name] {
			p.completedNodes[node.Step.Name] = true
			p.completed++
		}
	}
}

// UpdateStatus updates the overall DAG status.
func (p *SimpleProgressDisplay) UpdateStatus(status *execution.DAGRunStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = status.Status
}

// SetDAGRunInfo sets the DAG run ID and parameters.
func (p *SimpleProgressDisplay) SetDAGRunInfo(dagRunID, _ string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dagRunID = dagRunID
}

func (p *SimpleProgressDisplay) run() {
	defer close(p.done)

	// Print header
	p.printHeader()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			p.printFinal()
			return
		case <-ticker.C:
			p.render()
		}
	}
}

func (p *SimpleProgressDisplay) printHeader() {
	p.mu.Lock()
	defer p.mu.Unlock()

	dagName := "unknown"
	if p.dag != nil {
		dagName = p.dag.Name
	}

	runID := p.dagRunID
	if runID == "" {
		runID = "..."
	}

	fmt.Fprintf(os.Stderr, "Running %s (run id = %s)\n", dagName, runID)
}

func (p *SimpleProgressDisplay) render() {
	p.mu.Lock()
	defer p.mu.Unlock()

	spinner := spinnerFrames[p.spinnerIndex%len(spinnerFrames)]
	p.spinnerIndex++

	percent := 0
	if p.total > 0 {
		percent = (p.completed * 100) / p.total
	}

	// Use \r to overwrite the line, pad with spaces to clear previous content
	fmt.Fprintf(os.Stderr, "\r%s %d%% (%d/%d steps)   ", spinner, percent, p.completed, p.total)
}

func (p *SimpleProgressDisplay) printFinal() {
	p.mu.Lock()
	defer p.mu.Unlock()

	percent := 0
	if p.total > 0 {
		percent = (p.completed * 100) / p.total
	}

	icon := "✓"
	if p.status == core.Failed || p.status == core.Aborted {
		icon = "✗"
	}

	// Clear line and print final status
	fmt.Fprintf(os.Stderr, "\r%s %d%% (%d/%d steps)   \n", icon, percent, p.completed, p.total)
}

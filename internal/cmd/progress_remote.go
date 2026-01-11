package cmd

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/output"
	"github.com/dagu-org/dagu/internal/proto/convert"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"golang.org/x/term"
)

var remoteSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// RemoteProgressDisplay displays progress for distributed DAG execution.
// It polls the coordinator for status updates and renders a progress display
// similar to the local SimpleProgressDisplay.
type RemoteProgressDisplay struct {
	dag          *core.DAG
	dagRunID     string
	startTime    time.Time
	total        int
	colorEnabled bool
	isTTY        bool

	mu           sync.Mutex
	completed    int
	spinnerIndex int
	lastStatus   *execution.DAGRunStatus
	stopped      bool
}

// NewRemoteProgressDisplay creates a new remote progress display.
func NewRemoteProgressDisplay(dag *core.DAG, dagRunID string) *RemoteProgressDisplay {
	total := 0
	if dag != nil {
		total = len(dag.Steps)
	}
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))
	return &RemoteProgressDisplay{
		dag:          dag,
		dagRunID:     dagRunID,
		total:        total,
		colorEnabled: isTTY,
		isTTY:        isTTY,
		startTime:    time.Now(),
	}
}

// Start prints the initial header.
func (p *RemoteProgressDisplay) Start() {
	p.printHeader()
}

// Update updates the display with new status from coordinator.
func (p *RemoteProgressDisplay) Update(protoStatus *coordinatorv1.DAGRunStatusProto) {
	if protoStatus == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Convert proto to execution status
	status := convert.ProtoToDAGRunStatus(protoStatus)
	p.lastStatus = status

	// Count completed nodes
	p.completed = 0
	for _, node := range status.Nodes {
		if isRemoteNodeComplete(node.Status) {
			p.completed++
		}
	}

	p.render()
}

// Stop stops the display and prints final status.
func (p *RemoteProgressDisplay) Stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	status := p.lastStatus
	p.mu.Unlock()

	p.printFinal(status)
}

// GetLastStatus returns the last known status.
func (p *RemoteProgressDisplay) GetLastStatus() *execution.DAGRunStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastStatus
}

// PrintSummary prints the final tree summary.
func (p *RemoteProgressDisplay) PrintSummary() {
	p.mu.Lock()
	status := p.lastStatus
	dag := p.dag
	p.mu.Unlock()

	if status == nil || dag == nil {
		return
	}

	// Create renderer with remote-appropriate config (no stdout/stderr since logs are remote)
	config := output.Config{
		ColorEnabled:   term.IsTerminal(int(os.Stdout.Fd())),
		ShowStdout:     false, // No remote log access
		ShowStderr:     false, // No remote log access
		MaxOutputLines: 0,
		MaxWidth:       output.DefaultMaxWidth,
	}

	renderer := output.NewRenderer(config)
	summary := renderer.RenderDAGStatus(dag, status)

	_, _ = os.Stdout.WriteString(summary)
	_, _ = os.Stdout.WriteString("\n")
	_ = os.Stdout.Sync()
}

func (p *RemoteProgressDisplay) printHeader() {
	dagName := "unknown"
	if p.dag != nil {
		dagName = p.dag.Name
	}

	if p.isTTY {
		fmt.Fprintf(os.Stderr, "▶ %s %s\n", dagName, p.gray("("+p.dagRunID+")"))
	} else {
		// Non-TTY: just print once
		fmt.Fprintf(os.Stderr, "Started: %s (%s)\n", dagName, p.dagRunID)
	}
}

func (p *RemoteProgressDisplay) render() {
	if !p.isTTY {
		return // No inline updates for non-TTY
	}

	spinner := remoteSpinnerFrames[p.spinnerIndex%len(remoteSpinnerFrames)]
	p.spinnerIndex++

	percent := 0
	if p.total > 0 {
		percent = (p.completed * 100) / p.total
	}

	elapsed := stringutil.FormatDuration(time.Since(p.startTime))

	// Use \r to overwrite the line, pad with spaces to clear previous content
	fmt.Fprintf(os.Stderr, "\r%s %d%% (%d/%d steps) %s   ", spinner, percent, p.completed, p.total, p.gray(elapsed))
}

func (p *RemoteProgressDisplay) printFinal(status *execution.DAGRunStatus) {
	if !p.isTTY {
		return
	}

	p.mu.Lock()
	percent := 0
	if p.total > 0 {
		percent = (p.completed * 100) / p.total
	}
	p.mu.Unlock()

	icon := "✓"
	if status != nil && (status.Status == core.Failed || status.Status == core.Aborted) {
		icon = "✗"
	}

	elapsed := stringutil.FormatDuration(time.Since(p.startTime))

	// Clear line and print final status
	fmt.Fprintf(os.Stderr, "\r%s %d%% (%d/%d steps) %s   \n", icon, percent, p.completed, p.total, p.gray(elapsed))
}

// gray returns text in gray color if color is enabled.
func (p *RemoteProgressDisplay) gray(s string) string {
	if !p.colorEnabled {
		return s
	}
	return "\033[38;5;245m" + s + "\033[0m"
}

// isRemoteNodeComplete checks if a node has completed (success, failure, or skipped).
func isRemoteNodeComplete(status core.NodeStatus) bool {
	return status == core.NodeSucceeded ||
		status == core.NodeFailed ||
		status == core.NodeSkipped ||
		status == core.NodeAborted ||
		status == core.NodePartiallySucceeded
}

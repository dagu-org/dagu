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

	mu                      sync.Mutex
	completed               int
	spinnerIndex            int
	lastStatus              *execution.DAGRunStatus
	workerID                string
	headerUpdatedWithWorker bool
	stopped                 bool
	stopCh                  chan struct{}
	wg                      sync.WaitGroup
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
		stopCh:       make(chan struct{}),
	}
}

// Start prints the initial header and starts the animation loop.
func (p *RemoteProgressDisplay) Start() {
	p.printHeader()

	if p.isTTY {
		p.wg.Add(1)
		go p.animationLoop()
	}
}

// animationLoop runs a fast ticker to update the spinner animation.
func (p *RemoteProgressDisplay) animationLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.mu.Lock()
			if !p.stopped {
				p.render()
			}
			p.mu.Unlock()
		}
	}
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

	// Capture worker ID if available and update header
	if status.WorkerID != "" && p.workerID == "" {
		p.workerID = status.WorkerID
		// Update header to show worker assignment
		if p.isTTY && !p.headerUpdatedWithWorker {
			p.updateHeaderWithWorker()
			p.headerUpdatedWithWorker = true
		}
	}

	// Count completed nodes
	p.completed = 0
	for _, node := range status.Nodes {
		if node.Status.IsDone() {
			p.completed++
		}
	}
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

	// Stop the animation loop
	close(p.stopCh)
	p.wg.Wait()

	p.printFinal(status)
}

// GetLastStatus returns the last known status.
func (p *RemoteProgressDisplay) GetLastStatus() *execution.DAGRunStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastStatus
}

// SetCancelled updates the status to Aborted when cancellation is requested.
func (p *RemoteProgressDisplay) SetCancelled() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastStatus != nil {
		p.lastStatus.Status = core.Aborted
	}
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

	// Create renderer config - logs are synced from remote workers
	config := output.DefaultConfig()
	config.ColorEnabled = term.IsTerminal(int(os.Stdout.Fd()))
	config.MaxOutputLines = 10

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

// updateHeaderWithWorker reprints the header line with worker info.
// Must be called with mu held and only in TTY mode.
func (p *RemoteProgressDisplay) updateHeaderWithWorker() {
	dagName := "unknown"
	if p.dag != nil {
		dagName = p.dag.Name
	}

	// ANSI escape sequences:
	// \r       - move to beginning of current line
	// \033[K   - clear from cursor to end of line
	// \033[1A  - move cursor up 1 line
	// \033[1B  - move cursor down 1 line

	// Clear current progress line, move up to header, clear header, print new header, move back down
	fmt.Fprintf(os.Stderr, "\r\033[K\033[1A\r\033[K▶ %s %s %s\n", dagName, p.gray("("+p.dagRunID+")"), p.gray("→ "+p.workerID))
}

// completedAndPercent returns capped completed count and percentage.
// Must be called with mu held.
func (p *RemoteProgressDisplay) completedAndPercent() (completed, percent int) {
	completed = p.completed
	if completed > p.total {
		completed = p.total
	}
	if p.total > 0 {
		percent = (completed * 100) / p.total
	}
	return completed, percent
}

// render must be called with mu held.
func (p *RemoteProgressDisplay) render() {
	if !p.isTTY {
		return
	}

	spinner := stringutil.SpinnerFrames[p.spinnerIndex%len(stringutil.SpinnerFrames)]
	p.spinnerIndex++

	completed, percent := p.completedAndPercent()
	elapsed := stringutil.FormatDuration(time.Since(p.startTime))

	// Build worker info if available
	workerInfo := ""
	if p.workerID != "" {
		workerInfo = " → " + p.workerID
	}

	// Use \r to overwrite the line, pad with spaces to clear previous content
	fmt.Fprintf(os.Stderr, "\r%s %d%% (%d/%d steps) %s%s   ", spinner, percent, completed, p.total, p.gray(elapsed), p.gray(workerInfo))
}

func (p *RemoteProgressDisplay) printFinal(status *execution.DAGRunStatus) {
	p.mu.Lock()
	completed, percent := p.completedAndPercent()
	workerID := p.workerID
	p.mu.Unlock()

	icon := "✓"
	statusText := "completed"
	if status != nil && (status.Status == core.Failed || status.Status == core.Aborted) {
		icon = "✗"
		statusText = output.StatusText(status.Status)
	}

	elapsed := stringutil.FormatDuration(time.Since(p.startTime))

	// Build worker info if available
	workerInfo := ""
	if workerID != "" {
		workerInfo = " → " + workerID
	}

	if p.isTTY {
		fmt.Fprintf(os.Stderr, "\r%s %d%% (%d/%d steps) %s%s   \n", icon, percent, completed, p.total, p.gray(elapsed), p.gray(workerInfo))
	} else {
		if workerID != "" {
			fmt.Fprintf(os.Stderr, "Finished: %s (%d/%d steps) [%s] worker=%s\n", statusText, completed, p.total, elapsed, workerID)
		} else {
			fmt.Fprintf(os.Stderr, "Finished: %s (%d/%d steps) [%s]\n", statusText, completed, p.total, elapsed)
		}
	}
}

// gray returns text in gray color if color is enabled.
func (p *RemoteProgressDisplay) gray(s string) string {
	if !p.colorEnabled {
		return s
	}
	return "\033[38;5;245m" + s + "\033[0m"
}

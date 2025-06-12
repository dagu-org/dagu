package agent

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/fatih/color"
	"golang.org/x/term"
)

// ProgressDisplay manages the real-time progress display for DAG execution
type ProgressDisplay struct {
	mu        sync.Mutex
	writer    io.Writer
	dag       *digraph.DAG
	status    *models.DAGRunStatus
	nodes     map[string]*nodeProgress
	startTime time.Time
	ticker    *time.Ticker
	done      chan bool
	isEnabled bool
	isTTY     bool

	// DAG run information
	dagRunID string
	params   string

	// Display options
	showChildDetails bool
	accentColor      *color.Color
	spinnerFrames    []string
	spinnerIndex     int

	// Terminal dimensions
	termWidth  int
	termHeight int
}

type nodeProgress struct {
	node      *models.Node
	startTime time.Time
	endTime   time.Time
	status    scheduler.NodeStatus
	children  []models.ChildDAGRun
}

// NewProgressDisplay creates a new progress display
func NewProgressDisplay(writer io.Writer, dag *digraph.DAG) *ProgressDisplay {
	if writer == nil {
		writer = os.Stderr // Use stderr by default for progress display
	}

	// Check if stderr is connected to a terminal
	// We check stderr instead of the writer because progress should be
	// shown when stderr is a TTY, regardless of where stdout is directed
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))

	// Get terminal dimensions from stderr
	termWidth, termHeight := 80, 24 // defaults
	if isTTY {
		if w, h, err := term.GetSize(int(os.Stderr.Fd())); err == nil {
			termWidth, termHeight = w, h
		}
	}

	pd := &ProgressDisplay{
		writer:           writer,
		dag:              dag,
		nodes:            make(map[string]*nodeProgress),
		startTime:        time.Now(),
		isEnabled:        isTTY, // Only enable for TTY
		isTTY:            isTTY,
		showChildDetails: true,
		accentColor:      color.New(color.FgCyan),
		spinnerFrames:    []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		termWidth:        termWidth,
		termHeight:       termHeight,
	}

	// Initialize all nodes from the DAG steps
	if dag != nil {
		for _, step := range dag.Steps {
			pd.nodes[step.Name] = &nodeProgress{
				node: &models.Node{
					Step:       step,
					Status:     scheduler.NodeStatusNone,
					StartedAt:  "-",
					FinishedAt: "-",
				},
				status: scheduler.NodeStatusNone,
			}
		}
	}

	return pd
}

// Start begins the progress display
func (pd *ProgressDisplay) Start() {
	if !pd.isEnabled {
		return
	}

	pd.done = make(chan bool)
	pd.ticker = time.NewTicker(100 * time.Millisecond)

	// Clear screen and hide cursor
	fmt.Fprint(pd.writer, "\033[?25l") // Hide cursor
	fmt.Fprint(pd.writer, "\033[2J")   // Clear screen
	fmt.Fprint(pd.writer, "\033[H")    // Move to top

	go pd.runDisplayLoop()
}

// Stop stops the progress display
func (pd *ProgressDisplay) Stop() {
	if !pd.isEnabled || pd.done == nil {
		return
	}

	close(pd.done)
	if pd.ticker != nil {
		pd.ticker.Stop()
	}

	// Wait a bit to ensure the last render completes
	time.Sleep(50 * time.Millisecond)

	// Clear screen and render final state
	fmt.Fprint(pd.writer, "\033[H\033[2J")

	// Render the final state with all content
	pd.mu.Lock()
	pd.renderHeader()
	pd.renderProgressBar()
	pd.renderCurrentlyRunning()
	pd.renderRecentlyCompleted()
	pd.renderQueued()
	pd.renderChildDAGs()
	pd.renderFooter()
	pd.mu.Unlock()

	// Show cursor
	fmt.Fprint(pd.writer, "\033[?25h")

	// Add extra newlines at the bottom to ensure any following
	// output (like "exit status 1") appears below the display
	fmt.Fprint(pd.writer, "\n\n\n\n")
}

// UpdateNode updates the progress for a specific node
func (pd *ProgressDisplay) UpdateNode(node *models.Node) {
	if !pd.isEnabled {
		return
	}

	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Get existing node progress or create new one if not found
	np, exists := pd.nodes[node.Step.Name]
	if !exists {
		// This shouldn't happen if DAG was properly initialized, but handle it gracefully
		np = &nodeProgress{}
		pd.nodes[node.Step.Name] = np
	}

	// Update the node data
	np.node = node
	np.status = node.Status

	if node.StartedAt != "" && node.StartedAt != "-" {
		if t, err := stringutil.ParseTime(node.StartedAt); err == nil {
			np.startTime = t
		}
	}

	if node.FinishedAt != "" && node.FinishedAt != "-" {
		if t, err := stringutil.ParseTime(node.FinishedAt); err == nil {
			np.endTime = t
		}
	}

	if node.Children != nil {
		np.children = node.Children
	}
}

// UpdateStatus updates the overall DAG status
func (pd *ProgressDisplay) UpdateStatus(status *models.DAGRunStatus) {
	if !pd.isEnabled {
		return
	}

	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.status = status

	// Update DAG run info from status if available
	if status != nil {
		pd.dagRunID = status.DAGRunID
		pd.params = status.Params
	}
}

func (pd *ProgressDisplay) runDisplayLoop() {
	for {
		select {
		case <-pd.done:
			return
		case <-pd.ticker.C:
			pd.render()
		}
	}
}

func (pd *ProgressDisplay) render() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Update terminal dimensions in case of resize
	if pd.isTTY {
		if w, h, err := term.GetSize(int(os.Stderr.Fd())); err == nil {
			pd.termWidth, pd.termHeight = w, h
		}
	}

	// Increment spinner
	pd.spinnerIndex = (pd.spinnerIndex + 1) % len(pd.spinnerFrames)

	// Clear screen and reset cursor
	fmt.Fprint(pd.writer, "\033[H\033[2J")

	// Render header
	pd.renderHeader()

	// Render progress bar
	pd.renderProgressBar()

	// Render sections
	pd.renderCurrentlyRunning()
	pd.renderRecentlyCompleted()
	pd.renderQueued()
	pd.renderChildDAGs()

	// Render footer
	pd.renderFooter()
}

func (pd *ProgressDisplay) renderHeader() {
	elapsed := time.Since(pd.startTime)

	// Build status indicator
	statusStr := pd.formatStatus(pd.getOverallStatus())

	// Calculate available width
	boxWidth := pd.termWidth
	if boxWidth > 100 {
		boxWidth = 100 // Cap max width for readability
	}
	innerWidth := boxWidth - 2 // Account for box borders

	// Build header
	dagName := pd.dag.Name
	if len(dagName) > 30 {
		dagName = dagName[:27] + "..."
	}
	header := fmt.Sprintf(" DAG: %s ", dagName)

	// Build the time part (no colors)
	timePart := fmt.Sprintf("Started: %s | Elapsed: %s",
		pd.startTime.Format("15:04:05"),
		pd.formatDuration(elapsed))

	// Calculate the plain text parts for spacing
	statusPrefix := "Status: "
	statusValuePlain := stripANSI(statusStr)

	// Truncate time part if needed to fit within available width
	maxTimePartLen := innerWidth - utf8.RuneCountInString(statusPrefix) -
		utf8.RuneCountInString(statusValuePlain) - utf8.RuneCountInString(" | ") - 4
	if maxTimePartLen < 20 && utf8.RuneCountInString(timePart) > maxTimePartLen {
		// Just show elapsed time if space is tight
		timePart = fmt.Sprintf("Elapsed: %s", pd.formatDuration(elapsed))
	}

	// Calculate available space for the middle padding
	// Use rune count for visual width instead of byte length
	usedSpace := utf8.RuneCountInString(statusPrefix) + utf8.RuneCountInString(statusValuePlain) +
		utf8.RuneCountInString(" | ") + utf8.RuneCountInString(timePart) + 2 // 2 for space padding
	middlePadding := innerWidth - usedSpace
	if middlePadding < 1 {
		middlePadding = 1
	}

	// Build the status line with calculated padding
	// If the content is still too wide, truncate the time part
	if usedSpace > innerWidth {
		// Recalculate with truncated time
		availableForTime := innerWidth - utf8.RuneCountInString(statusPrefix) -
			utf8.RuneCountInString(statusValuePlain) - utf8.RuneCountInString(" | ") - 5
		if availableForTime > 0 {
			timeRunes := []rune(timePart)
			if len(timeRunes) > availableForTime-3 && availableForTime > 3 && len(timeRunes) > 0 {
				println(fmt.Sprintf("Truncating time part: %s availableForTime=%d", timePart, availableForTime))
				timePart = string(timeRunes[:availableForTime-3]) + "..."
			}
		} else {
			timePart = "..."
		}
		middlePadding = 1
	}

	statusLine := fmt.Sprintf(" %s%s%s | %s ",
		statusPrefix,
		statusStr, // This includes color codes
		strings.Repeat(" ", middlePadding),
		timePart)

	// Apply color to header
	coloredHeader := pd.accentColor.Sprint(header)

	// Calculate header padding based on plain text length
	// We need to account for the ─ after ┌ and before ┐
	headerPadding := innerWidth - utf8.RuneCountInString(stripANSI(header)) - 2
	if headerPadding < 0 {
		headerPadding = 0
	}

	// Render the box
	fmt.Fprintf(pd.writer, "┌─%s%s─┐\n",
		coloredHeader,
		strings.Repeat("─", headerPadding))
	fmt.Fprintln(pd.writer, "│"+statusLine+"│")

	// Add Run ID line
	if pd.dagRunID != "" {
		runIDStr := fmt.Sprintf("Run ID: %s", pd.truncateString(pd.dagRunID, innerWidth-12))
		runIDLine := fmt.Sprintf(" %s%s ", runIDStr, strings.Repeat(" ", innerWidth-utf8.RuneCountInString(runIDStr)-2))
		fmt.Fprintln(pd.writer, "│"+runIDLine+"│")
	}

	// Add Params line if present
	if pd.params != "" {
		paramsStr := fmt.Sprintf("Params: %s", pd.truncateString(pd.params, innerWidth-12))
		paramsLine := fmt.Sprintf(" %s%s ", paramsStr, strings.Repeat(" ", innerWidth-utf8.RuneCountInString(paramsStr)-2))
		fmt.Fprintln(pd.writer, "│"+paramsLine+"│")
	}

	fmt.Fprintf(pd.writer, "└%s┘\n\n", strings.Repeat("─", innerWidth))
}

func (pd *ProgressDisplay) renderProgressBar() {
	completed := 0
	total := len(pd.nodes)

	for _, np := range pd.nodes {
		if np.status == scheduler.NodeStatusSuccess ||
			np.status == scheduler.NodeStatusError ||
			np.status == scheduler.NodeStatusSkipped ||
			np.status == scheduler.NodeStatusCancel {
			completed++
		}
	}

	if total == 0 {
		return
	}

	percentage := (completed * 100) / total
	barWidth := 40
	filled := (percentage * barWidth) / 100

	// Build progress bar
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Fprintf(pd.writer, "Progress: %s %3d%% (%d/%d steps)\n\n",
		pd.colorizeProgressBar(bar, percentage),
		percentage, completed, total)
}

func (pd *ProgressDisplay) renderCurrentlyRunning() {
	running := pd.getNodesByStatus(scheduler.NodeStatusRunning)
	if len(running) == 0 {
		return
	}

	fmt.Fprintln(pd.writer, color.New(color.Bold).Sprint("Currently Running:"))

	for _, np := range running {
		elapsed := time.Since(np.startTime)
		spinner := pd.spinnerFrames[pd.spinnerIndex]

		fmt.Fprintf(pd.writer, "  %s %s %s\n",
			color.New(color.FgHiGreen).Sprint(spinner),
			pd.truncateString(np.node.Step.Name, 30),
			color.New(color.Faint).Sprintf("[Running for %s]", pd.formatDuration(elapsed)))
	}
	fmt.Fprintln(pd.writer)
}

func (pd *ProgressDisplay) renderRecentlyCompleted() {
	// Get completed nodes sorted by completion time
	completed := pd.getCompletedNodes()
	if len(completed) == 0 {
		return
	}

	// Show only recent ones
	maxShow := 5
	if len(completed) > maxShow {
		completed = completed[len(completed)-maxShow:]
	}

	fmt.Fprintln(pd.writer, color.New(color.Bold).Sprint("Recently Completed:"))

	for _, np := range completed {
		statusIcon := pd.getStatusIcon(np.status)
		duration := ""
		if !np.endTime.IsZero() && !np.startTime.IsZero() {
			duration = fmt.Sprintf("[%s]", pd.formatDuration(np.endTime.Sub(np.startTime)))
		}

		line := fmt.Sprintf("  %s %s %s",
			statusIcon,
			pd.truncateString(np.node.Step.Name, 30),
			color.New(color.Faint).Sprint(duration))

		if np.status == scheduler.NodeStatusError && np.node.Error != "" {
			// Calculate available space for error message
			// Account for the line content already shown
			lineLen := 2 + 1 + 1 + 30 + 1 + len(duration) + 8 // Approximate
			availableSpace := pd.termWidth - lineLen - 10     // Leave some margin
			if availableSpace < 20 {
				availableSpace = 20
			}
			errorMsg := pd.truncateString(np.node.Error, availableSpace)
			line += color.RedString(" Error: %s", errorMsg)
		}

		fmt.Fprintln(pd.writer, line)
	}
	fmt.Fprintln(pd.writer)
}

func (pd *ProgressDisplay) renderQueued() {
	queued := pd.getNodesByStatus(scheduler.NodeStatusNone)
	if len(queued) == 0 {
		return
	}

	maxShow := 3
	fmt.Fprintln(pd.writer, color.New(color.Bold).Sprint("Queued:"))

	for i, np := range queued {
		if i >= maxShow {
			fmt.Fprintf(pd.writer, "  %s ... and %d more\n",
				color.New(color.Faint).Sprint("○"),
				len(queued)-maxShow)
			break
		}
		fmt.Fprintf(pd.writer, "  %s %s\n",
			color.New(color.Faint).Sprint("○"),
			pd.truncateString(np.node.Step.Name, 30))
	}
	fmt.Fprintln(pd.writer)
}

func (pd *ProgressDisplay) renderChildDAGs() {
	childNodes := pd.getNodesWithChildren()
	if len(childNodes) == 0 {
		return
	}

	fmt.Fprintln(pd.writer, color.New(color.Bold).Sprint("Child DAGs:"))

	for _, np := range childNodes {
		if len(np.children) == 1 {
			// Single child DAG
			child := np.children[0]
			fmt.Fprintf(pd.writer, "  ▸ %s → %s\n",
				pd.truncateString(np.node.Step.Name, 20),
				pd.formatChildStatus(child))
		} else {
			// Parallel execution
			// For parallel execution, we'll show the count and basic info
			// Actual status tracking would need to be implemented separately
			total := len(np.children)

			// Show a simplified view for now
			statusInfo := fmt.Sprintf("(%d child DAGs)", total)

			fmt.Fprintf(pd.writer, "  ▸ %s %s %s\n",
				pd.truncateString(np.node.Step.Name, 20),
				statusInfo,
				pd.getStatusIcon(np.status))
		}
	}
	fmt.Fprintln(pd.writer)
}

func (pd *ProgressDisplay) renderFooter() {
	hint := color.New(color.Faint).Sprint("Press Ctrl+C to stop")
	fmt.Fprintln(pd.writer, hint)
}

// Helper functions

func (pd *ProgressDisplay) getOverallStatus() scheduler.Status {
	if pd.status != nil {
		return pd.status.Status
	}
	return scheduler.StatusRunning
}

func (pd *ProgressDisplay) formatStatus(status scheduler.Status) string {
	switch status {
	case scheduler.StatusSuccess:
		return color.GreenString("Success ✓")
	case scheduler.StatusError:
		return color.RedString("Failed ✗")
	case scheduler.StatusRunning:
		return color.New(color.FgHiGreen).Sprint("Running ●")
	case scheduler.StatusCancel:
		return color.YellowString("Cancelled ⚠")
	case scheduler.StatusQueued:
		return color.BlueString("Queued ●")
	default:
		return color.New(color.Faint).Sprint("Not Started ○")
	}
}

func (pd *ProgressDisplay) getStatusIcon(status scheduler.NodeStatus) string {
	switch status {
	case scheduler.NodeStatusSuccess:
		return color.GreenString("✓")
	case scheduler.NodeStatusError:
		return color.RedString("✗")
	case scheduler.NodeStatusRunning:
		return color.New(color.FgHiGreen).Sprint("●")
	case scheduler.NodeStatusCancel:
		return color.YellowString("⚠")
	case scheduler.NodeStatusSkipped:
		return color.New(color.Faint).Sprint("⊘")
	default:
		return color.New(color.Faint).Sprint("○")
	}
}

func (pd *ProgressDisplay) formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func (pd *ProgressDisplay) colorizeProgressBar(bar string, percentage int) string {
	if percentage >= 80 {
		return color.GreenString(bar)
	} else if percentage >= 50 {
		return pd.accentColor.Sprint(bar)
	}
	return bar
}

func (pd *ProgressDisplay) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (pd *ProgressDisplay) formatChildStatus(child models.ChildDAGRun) string {
	// For now, just show the child DAG run ID and params
	// Status tracking for child DAGs would need to be retrieved separately
	params := ""
	if child.Params != "" {
		params = color.New(color.Faint).Sprintf(" [%s]", pd.truncateString(child.Params, 20))
	}
	dagRunID := pd.truncateString(child.DAGRunID, 15)
	return fmt.Sprintf("%s%s", dagRunID, params)
}

func (pd *ProgressDisplay) getNodesByStatus(status scheduler.NodeStatus) []*nodeProgress {
	var nodes []*nodeProgress
	for _, np := range pd.nodes {
		if np.status == status {
			nodes = append(nodes, np)
		}
	}

	// Sort by start time (earliest first) for deterministic ordering
	// If start times are equal, sort by step name
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[i].startTime.After(nodes[j].startTime) ||
				(nodes[i].startTime.Equal(nodes[j].startTime) &&
					nodes[i].node.Step.Name > nodes[j].node.Step.Name) {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}

	return nodes
}

func (pd *ProgressDisplay) getCompletedNodes() []*nodeProgress {
	var nodes []*nodeProgress
	for _, np := range pd.nodes {
		if np.status == scheduler.NodeStatusSuccess ||
			np.status == scheduler.NodeStatusError ||
			np.status == scheduler.NodeStatusSkipped ||
			np.status == scheduler.NodeStatusCancel {
			nodes = append(nodes, np)
		}
	}

	// Sort by completion time
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			switch {
			case nodes[i].endTime.IsZero() && !nodes[j].endTime.IsZero():
				// If i has no end time, it comes after j
				nodes[i], nodes[j] = nodes[j], nodes[i]
			case !nodes[i].endTime.IsZero() && nodes[j].endTime.IsZero():
				// If j has no end time, it comes after i
				// No swap needed
			case nodes[i].endTime.Before(nodes[j].endTime):
				// If i ends before j, i comes first
				// No swap needed
			case nodes[i].endTime.After(nodes[j].endTime):
				// If i ends after j, swap them
				nodes[i], nodes[j] = nodes[j], nodes[i]
			default:
				// If end times are equal, sort by step name
				if nodes[i].node.Step.Name > nodes[j].node.Step.Name {
					nodes[i], nodes[j] = nodes[j], nodes[i]
				}
			}
		}
	}

	return nodes
}

func (pd *ProgressDisplay) getNodesWithChildren() []*nodeProgress {
	var nodes []*nodeProgress
	for _, np := range pd.nodes {
		if len(np.children) > 0 {
			nodes = append(nodes, np)
		}
	}

	// Sort by step name for deterministic ordering
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[i].node.Step.Name > nodes[j].node.Step.Name {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}

	return nodes
}

// stripANSI removes ANSI color codes from a string
func stripANSI(s string) string {
	// Remove all ANSI escape sequences
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i++ // Skip the '['
			continue
		}
		if inEscape {
			// Skip until we find a letter that ends the escape sequence
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

// SetAccentColor allows customizing the main accent color
func (pd *ProgressDisplay) SetAccentColor(c *color.Color) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if c != nil {
		pd.accentColor = c
	}
}

// SetDAGRunInfo sets the DAG run ID and parameters
func (pd *ProgressDisplay) SetDAGRunInfo(dagRunID, params string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.dagRunID = dagRunID
	pd.params = params
}

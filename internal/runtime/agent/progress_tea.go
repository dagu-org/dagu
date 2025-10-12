package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/status"
)

// nodeProgress represents the progress state of a single node
type nodeProgress struct {
	node      *execution.Node
	startTime time.Time
	endTime   time.Time
	status    status.NodeStatus
	children  []execution.ChildDAGRun
}

// Message types for Bubble Tea
type (
	// TickMsg is sent periodically to update the display
	TickMsg time.Time

	// NodeUpdateMsg is sent when a node's status changes
	NodeUpdateMsg struct {
		Node *execution.Node
	}

	// StatusUpdateMsg is sent when the overall DAG status changes
	StatusUpdateMsg struct {
		Status *execution.DAGRunStatus
	}

	// FinalizeMsg is sent when the display should stop
	FinalizeMsg struct{}
)

// ProgressModel represents the Bubble Tea model for progress display
type ProgressModel struct {
	// DAG information
	dag      *core.DAG
	status   *execution.DAGRunStatus
	dagRunID string
	params   string

	// Node tracking
	nodes map[string]*nodeProgress

	// Display state
	startTime        time.Time
	finishTime       time.Time
	spinner          spinner.Model
	width            int
	height           int
	finalized        bool
	showChildDetails bool

	// Styles
	accentColor      lipgloss.Color
	headerStyle      lipgloss.Style
	progressBarStyle lipgloss.Style
	sectionStyle     lipgloss.Style
	errorStyle       lipgloss.Style
	successStyle     lipgloss.Style
	runningStyle     lipgloss.Style
	faintStyle       lipgloss.Style
	boldStyle        lipgloss.Style
}

// NewProgressModel creates a new progress model for Bubble Tea
func NewProgressModel(dag *core.DAG) ProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    time.Second / 10,
	}

	// Initialize styles
	accentColor := lipgloss.Color("6")

	m := ProgressModel{
		dag:              dag,
		nodes:            make(map[string]*nodeProgress),
		startTime:        time.Now(),
		spinner:          s,
		showChildDetails: true,
		accentColor:      accentColor,
		headerStyle:      lipgloss.NewStyle().Foreground(accentColor).Bold(true),
		sectionStyle:     lipgloss.NewStyle().Bold(true),
		errorStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		successStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		runningStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		faintStyle:       lipgloss.NewStyle().Faint(true),
		boldStyle:        lipgloss.NewStyle().Bold(true),
	}

	// Initialize all nodes from the DAG steps
	if dag != nil {
		for _, step := range dag.Steps {
			m.nodes[step.Name] = &nodeProgress{
				node: &execution.Node{
					Step:       step,
					Status:     status.NodeNone,
					StartedAt:  "-",
					FinishedAt: "-",
				},
				status: status.NodeNone,
			}
		}
	}

	return m
}

// Init initializes the model
func (m ProgressModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
	)
}

// Update handles messages
func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TickMsg:
		if m.finalized {
			return m, nil
		}
		return m, tickCmd()

	case NodeUpdateMsg:
		m.updateNode(msg.Node)
		return m, nil

	case StatusUpdateMsg:
		m.status = msg.Status
		if msg.Status != nil {
			m.dagRunID = msg.Status.DAGRunID
			m.params = msg.Status.Params
		}
		return m, nil

	case FinalizeMsg:
		m.finalized = true
		m.finishTime = time.Now()
		// Quit after a short delay to ensure final render
		return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return tea.Quit()
		})

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.finalized = true
			return m, tea.Quit
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the display
func (m ProgressModel) View() string {
	// Always use the same rendering logic whether finalized or not
	// This ensures the user sees the final state clearly

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Progress bar
	if bar := m.renderProgressBar(); bar != "" {
		sections = append(sections, bar)
	}

	// Currently running
	if running := m.renderCurrentlyRunning(); running != "" {
		sections = append(sections, running)
	}

	// Show different sections based on finalized state
	if m.finalized {
		// Show all completed steps when finalized
		if completed := m.renderAllCompleted(); completed != "" {
			sections = append(sections, completed)
		}
	} else {
		// Show recently completed and queued when running
		if completed := m.renderRecentlyCompleted(); completed != "" {
			sections = append(sections, completed)
		}

		if queued := m.renderQueued(); queued != "" {
			sections = append(sections, queued)
		}
	}

	// Child DAGs
	if children := m.renderChildDAGs(); children != "" {
		sections = append(sections, children)
	}

	// Footer
	sections = append(sections, m.renderFooter())

	return strings.Join(sections, "\n\n")
}

func (m *ProgressModel) updateNode(node *execution.Node) {
	np, exists := m.nodes[node.Step.Name]
	if !exists {
		np = &nodeProgress{}
		m.nodes[node.Step.Name] = np
	}

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

func (m ProgressModel) renderHeader() string {
	var elapsed time.Duration
	if m.finalized && !m.finishTime.IsZero() {
		elapsed = m.finishTime.Sub(m.startTime)
	} else {
		elapsed = time.Since(m.startTime)
	}

	// Build status indicator
	statusStr := m.formatStatus(m.getOverallStatus())

	// Box width calculation
	boxWidth := m.width
	if boxWidth == 0 {
		boxWidth = 80 // Default width if not set yet
	}
	if boxWidth > 100 {
		boxWidth = 100
	}
	if boxWidth < 40 {
		boxWidth = 40
	}
	innerWidth := boxWidth - 2

	// Build header
	dagName := m.dag.Name
	if len(dagName) > 30 {
		dagName = dagName[:27] + "..."
	}
	header := fmt.Sprintf(" DAG: %s ", dagName)

	// Build the time part
	timePart := fmt.Sprintf("Started: %s | Elapsed: %s",
		m.startTime.Format("15:04:05"),
		stringutil.FormatDuration(elapsed))

	// Build status line
	statusPrefix := "Status: "
	statusLine := fmt.Sprintf(" %s%s | %s ", statusPrefix, statusStr, timePart)

	// Apply styles
	headerStyled := m.headerStyle.Render(header)

	// Create box
	var box strings.Builder
	box.WriteString("┌─")
	box.WriteString(headerStyled)
	// Calculate header padding, ensuring it's never negative
	headerPadding := innerWidth - lipgloss.Width(header) - 2
	if headerPadding < 0 {
		headerPadding = 0
	}
	box.WriteString(strings.Repeat("─", headerPadding))
	box.WriteString("─┐\n")

	box.WriteString("│")
	box.WriteString(statusLine)
	// Calculate padding, ensuring it's never negative
	padding := innerWidth - lipgloss.Width(statusLine)
	if padding < 0 {
		padding = 0
	}
	box.WriteString(strings.Repeat(" ", padding))
	box.WriteString("│\n")

	// Add Run ID line
	if m.dagRunID != "" {
		runIDStr := fmt.Sprintf("Run ID: %s", truncateString(m.dagRunID, innerWidth-12))
		// Calculate padding for run ID line
		runIDPadding := innerWidth - len(runIDStr) - 2
		if runIDPadding < 0 {
			runIDPadding = 0
		}
		runIDLine := fmt.Sprintf(" %s%s ", runIDStr, strings.Repeat(" ", runIDPadding))
		box.WriteString("│")
		box.WriteString(runIDLine)
		box.WriteString("│\n")
	}

	// Add Params line
	if m.params != "" {
		paramsStr := fmt.Sprintf("Params: %s", truncateString(m.params, innerWidth-12))
		// Calculate padding for params line
		paramsPadding := innerWidth - len(paramsStr) - 2
		if paramsPadding < 0 {
			paramsPadding = 0
		}
		paramsLine := fmt.Sprintf(" %s%s ", paramsStr, strings.Repeat(" ", paramsPadding))
		box.WriteString("│")
		box.WriteString(paramsLine)
		box.WriteString("│\n")
	}

	box.WriteString("└")
	// Ensure innerWidth is never negative
	bottomPadding := innerWidth
	if bottomPadding < 0 {
		bottomPadding = 0
	}
	box.WriteString(strings.Repeat("─", bottomPadding))
	box.WriteString("┘")

	return box.String()
}

func (m ProgressModel) renderProgressBar() string {
	completed := 0
	total := len(m.nodes)

	for _, np := range m.nodes {
		if np.status == status.NodeSuccess ||
			np.status == status.NodeError ||
			np.status == status.NodeSkipped ||
			np.status == status.NodeCancel {
			completed++
		}
	}

	if total == 0 {
		return ""
	}

	percentage := (completed * 100) / total
	barWidth := 40
	if m.width < 60 {
		barWidth = 20
	}
	filled := (percentage * barWidth) / 100

	// Build progress bar
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Colorize based on percentage
	var barStyled string
	if percentage >= 80 {
		barStyled = m.successStyle.Render(bar)
	} else if percentage >= 50 {
		barStyled = lipgloss.NewStyle().Foreground(m.accentColor).Render(bar)
	} else {
		barStyled = bar
	}

	return fmt.Sprintf("Progress: %s %3d%% (%d/%d steps)",
		barStyled, percentage, completed, total)
}

func (m ProgressModel) renderCurrentlyRunning() string {
	running := m.getNodesByStatus(status.NodeRunning)
	if len(running) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, m.sectionStyle.Render("Currently Running:"))

	for _, np := range running {
		var elapsed time.Duration
		if !np.startTime.IsZero() {
			elapsed = time.Since(np.startTime)
		}
		spinner := m.spinner.View()

		line := fmt.Sprintf("  %s %s %s",
			m.runningStyle.Render(spinner),
			truncateString(np.node.Step.Name, 30),
			m.faintStyle.Render(fmt.Sprintf("[Running for %s]", stringutil.FormatDuration(elapsed))))

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m ProgressModel) renderRecentlyCompleted() string {
	completed := m.getCompletedNodes()
	if len(completed) == 0 {
		return ""
	}

	// Show only recent ones
	maxShow := 5
	if len(completed) > maxShow {
		completed = completed[len(completed)-maxShow:]
	}

	var lines []string
	lines = append(lines, m.sectionStyle.Render("Recently Completed:"))

	for _, np := range completed {
		statusIcon := m.getStatusIcon(np.status)
		duration := ""
		if !np.endTime.IsZero() && !np.startTime.IsZero() {
			duration = fmt.Sprintf("[%s]", stringutil.FormatDuration(np.endTime.Sub(np.startTime)))
		}

		line := fmt.Sprintf("  %s %s %s",
			statusIcon,
			truncateString(np.node.Step.Name, 30),
			m.faintStyle.Render(duration))

		if np.status == status.NodeError && np.node.Error != "" {
			availableSpace := m.width - 50
			if availableSpace < 20 {
				availableSpace = 20
			}
			errorMsg := truncateString(np.node.Error, availableSpace)
			line += m.errorStyle.Render(fmt.Sprintf(" Error: %s", errorMsg))
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m ProgressModel) renderAllCompleted() string {
	completed := m.getCompletedNodes()
	if len(completed) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, m.sectionStyle.Render("Completed Steps:"))

	for _, np := range completed {
		statusIcon := m.getStatusIcon(np.status)
		duration := ""
		if !np.endTime.IsZero() && !np.startTime.IsZero() {
			duration = fmt.Sprintf("[%s]", stringutil.FormatDuration(np.endTime.Sub(np.startTime)))
		}

		line := fmt.Sprintf("  %s %s %s",
			statusIcon,
			truncateString(np.node.Step.Name, 30),
			m.faintStyle.Render(duration))

		if np.status == status.NodeError && np.node.Error != "" {
			availableSpace := m.width - 50
			if availableSpace < 20 {
				availableSpace = 20
			}
			errorMsg := truncateString(np.node.Error, availableSpace)
			line += m.errorStyle.Render(fmt.Sprintf(" Error: %s", errorMsg))
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m ProgressModel) renderQueued() string {
	queued := m.getNodesByStatus(status.NodeNone)
	if len(queued) == 0 {
		return ""
	}

	maxShow := 3
	var lines []string
	lines = append(lines, m.sectionStyle.Render("Queued:"))

	for i, np := range queued {
		if i >= maxShow {
			lines = append(lines, fmt.Sprintf("  %s ... and %d more",
				m.faintStyle.Render("○"),
				len(queued)-maxShow))
			break
		}
		lines = append(lines, fmt.Sprintf("  %s %s",
			m.faintStyle.Render("○"),
			truncateString(np.node.Step.Name, 30)))
	}

	return strings.Join(lines, "\n")
}

func (m ProgressModel) renderChildDAGs() string {
	childNodes := m.getNodesWithChildren()
	if len(childNodes) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, m.sectionStyle.Render("Child DAGs:"))

	for _, np := range childNodes {
		if len(np.children) == 1 {
			// Single child DAG
			child := np.children[0]
			lines = append(lines, fmt.Sprintf("  ▸ %s → %s",
				truncateString(np.node.Step.Name, 20),
				m.formatChildStatus(child)))
		} else {
			// Parallel execution
			total := len(np.children)
			statusInfo := fmt.Sprintf("(%d child DAGs)", total)

			lines = append(lines, fmt.Sprintf("  ▸ %s %s %s",
				truncateString(np.node.Step.Name, 20),
				statusInfo,
				m.getStatusIcon(np.status)))
		}
	}

	return strings.Join(lines, "\n")
}

func (m ProgressModel) renderFooter() string {
	if m.finalized {
		st := m.getOverallStatus()
		switch st {
		case status.Success:
			return m.successStyle.Render("✓ Execution completed successfully")
		case status.Error:
			return m.errorStyle.Render("✗ Execution failed")
		case status.Cancel:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("⚠ Execution cancelled")
		default:
			return m.boldStyle.Render("Execution finished")
		}
	}
	return m.faintStyle.Render("Press Ctrl+C to stop")
}

func (m ProgressModel) getOverallStatus() status.Status {
	if m.status != nil {
		return m.status.Status
	}
	return status.Running
}

func (m ProgressModel) formatStatus(st status.Status) string {
	switch st {
	case status.Success:
		return m.successStyle.Render("Success ✓")
	case status.Error:
		return m.errorStyle.Render("Failed ✗")
	case status.Running:
		return m.runningStyle.Render("Running ●")
	case status.Cancel:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("Cancelled ⚠")
	case status.Queued:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Render("Queued ●")
	default:
		return m.faintStyle.Render("Not Started ○")
	}
}

func (m ProgressModel) getStatusIcon(s status.NodeStatus) string {
	switch s {
	case status.NodeSuccess:
		return m.successStyle.Render("✓")
	case status.NodeError:
		return m.errorStyle.Render("✗")
	case status.NodeRunning:
		return m.runningStyle.Render("●")
	case status.NodeCancel:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("⚠")
	case status.NodeSkipped:
		return m.faintStyle.Render("⊘")
	case status.NodePartialSuccess:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("◐")
	default:
		return m.faintStyle.Render("○")
	}
}

func (m ProgressModel) formatChildStatus(child execution.ChildDAGRun) string {
	params := ""
	if child.Params != "" {
		params = m.faintStyle.Render(fmt.Sprintf(" [%s]", truncateString(child.Params, 20)))
	}
	dagRunID := truncateString(child.DAGRunID, 15)
	return fmt.Sprintf("%s%s", dagRunID, params)
}

func (m ProgressModel) getNodesByStatus(s status.NodeStatus) []*nodeProgress {
	var nodes []*nodeProgress
	for _, np := range m.nodes {
		if np.status == s {
			nodes = append(nodes, np)
		}
	}

	// Sort by start time (earliest first) for deterministic ordering
	sortNodesByStartTime(nodes)
	return nodes
}

func (m ProgressModel) getCompletedNodes() []*nodeProgress {
	var nodes []*nodeProgress
	for _, np := range m.nodes {
		if np.status == status.NodeSuccess ||
			np.status == status.NodeError ||
			np.status == status.NodeSkipped ||
			np.status == status.NodeCancel {
			nodes = append(nodes, np)
		}
	}

	// Sort by completion time
	sortNodesByEndTime(nodes)
	return nodes
}

func (m ProgressModel) getNodesWithChildren() []*nodeProgress {
	var nodes []*nodeProgress
	for _, np := range m.nodes {
		if len(np.children) > 0 {
			nodes = append(nodes, np)
		}
	}

	// Sort by step name for deterministic ordering
	sortNodesByName(nodes)
	return nodes
}

// Helper functions

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func sortNodesByStartTime(nodes []*nodeProgress) {
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[i].startTime.After(nodes[j].startTime) ||
				(nodes[i].startTime.Equal(nodes[j].startTime) &&
					nodes[i].node.Step.Name > nodes[j].node.Step.Name) {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}
}

func sortNodesByEndTime(nodes []*nodeProgress) {
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			switch {
			case nodes[i].endTime.IsZero() && !nodes[j].endTime.IsZero():
				nodes[i], nodes[j] = nodes[j], nodes[i]
			case !nodes[i].endTime.IsZero() && nodes[j].endTime.IsZero():
				// No swap needed
			case nodes[i].endTime.After(nodes[j].endTime):
				nodes[i], nodes[j] = nodes[j], nodes[i]
			case nodes[i].endTime.Equal(nodes[j].endTime):
				if nodes[i].node.Step.Name > nodes[j].node.Step.Name {
					nodes[i], nodes[j] = nodes[j], nodes[i]
				}
			}
		}
	}
}

func sortNodesByName(nodes []*nodeProgress) {
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[i].node.Step.Name > nodes[j].node.Step.Name {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}
}

// ProgressTeaDisplay wraps the Bubble Tea program for the progress display
type ProgressTeaDisplay struct {
	program      *tea.Program
	model        ProgressModel
	useAltScreen bool
	done         chan struct{}
	finalOutput  string
}

// NewProgressTeaDisplay creates a new Bubble Tea-based progress display
func NewProgressTeaDisplay(dag *core.DAG) *ProgressTeaDisplay {
	model := NewProgressModel(dag)
	return &ProgressTeaDisplay{
		model:        model,
		useAltScreen: true,
		done:         make(chan struct{}),
	}
}

// Start initializes and runs the Bubble Tea program
func (p *ProgressTeaDisplay) Start() {
	// Manually manage alternate screen to keep it after exit
	p.program = tea.NewProgram(p.model)
	go func() {
		defer func() {
			// Ensure cursor is visible and mouse tracking is disabled if program crashes
			fmt.Print("\033[?25h")   // Show cursor
			fmt.Print("\033[?1000l") // Disable mouse tracking
			fmt.Print("\033[?1002l") // Disable mouse cell motion tracking
			fmt.Print("\033[?1003l") // Disable all mouse tracking
			fmt.Print("\033[?1006l") // Disable SGR mouse mode
			// DON'T exit alternate screen - keep the UI visible
			// Signal that the program has exited
			close(p.done)
		}()

		// Enter alternate screen manually
		fmt.Print("\033[?1049h") // Enter alternate screen
		fmt.Print("\033[2J")     // Clear screen
		fmt.Print("\033[H")      // Move cursor to home

		_, _ = p.program.Run()

		// Don't exit alternate screen - stay in it!
	}()
}

// Stop gracefully stops the display
func (p *ProgressTeaDisplay) Stop() {
	if p.program != nil {
		p.program.Send(FinalizeMsg{})
		// Wait for the program to exit
		<-p.done
		// UI stays visible in alternate screen buffer
	}
}

// UpdateNode sends a node update to the display
func (p *ProgressTeaDisplay) UpdateNode(node *execution.Node) {
	if p.program != nil {
		p.model.updateNode(node)
		p.program.Send(NodeUpdateMsg{Node: node})
	}
}

// UpdateStatus sends a status update to the display
func (p *ProgressTeaDisplay) UpdateStatus(status *execution.DAGRunStatus) {
	if p.program != nil {
		p.model.status = status
		p.program.Send(StatusUpdateMsg{Status: status})
	}
}

// SetDAGRunInfo sets the DAG run ID and parameters
func (p *ProgressTeaDisplay) SetDAGRunInfo(dagRunID, params string) {
	if p.program != nil {
		status := &execution.DAGRunStatus{
			DAGRunID: dagRunID,
			Params:   params,
		}
		p.program.Send(StatusUpdateMsg{Status: status})
	}
}

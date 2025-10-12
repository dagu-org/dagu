package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
)

func CmdStatus() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "status [flags] <DAG name>",
			Short: "Display the current status of a DAG-run",
			Long: `Show real-time status information for a specified DAG-run instance.

This command retrieves and displays the current execution status of a DAG-run,
including its state (running, completed, failed), process ID, and other relevant details.

Flags:
  --run-id string (optional) Unique identifier of the DAG-run to check.
                                 If not provided, it will show the status of the
                                 most recent DAG-run for the given name.

Example:
  dagu status --run-id=abc123 my_dag
  dagu status my_dag  # Shows status of the most recent DAG-run
`,
			Args: cobra.ExactArgs(1),
		}, statusFlags, runStatus,
	)
}

var statusFlags = []commandLineFlag{
	dagRunIDFlagStatus,
}

func runStatus(ctx *Context, args []string) error {
	dagRunID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to get dag-run ID: %w", err)
	}

	name, err := extractDAGName(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}
	attempt, err := extractAttemptID(ctx, name, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to extract attempt ID: %w", err)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from run data: %w", err)
	}

	dagStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status from attempt: %w", err)
	}

	if dagStatus.Status == status.Running {
		realtimeStatus, err := ctx.DAGRunMgr.GetCurrentStatus(ctx, dag, dagRunID)
		if err != nil {
			return fmt.Errorf("failed to retrieve current status: %w", err)
		}
		if realtimeStatus.DAGRunID == dagStatus.DAGRunID {
			dagStatus = realtimeStatus
		}
	}

	// Display detailed status information
	displayDetailedStatus(dag, dagStatus)

	return nil
}

// displayDetailedStatus renders a formatted table with DAG run information
func displayDetailedStatus(dag *digraph.DAG, dagStatus *models.DAGRunStatus) {
	// Create header with 80 character width
	fmt.Println()
	headerColor := color.New(color.FgCyan, color.Bold)

	// Create a boxed header similar to progress display
	fmt.Println(strings.Repeat("─", 80))
	title := "DAG Run Status Report"
	padding := (80 - len(title)) / 2
	fmt.Printf("%s%s%s\n",
		strings.Repeat(" ", padding),
		headerColor.Sprint(title),
		strings.Repeat(" ", 80-padding-len(title)))
	fmt.Println(strings.Repeat("─", 80))

	// Create overview table with fixed 80-character width
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	// Configure columns to ensure total width is 80 chars
	// Table borders: │ + space + col1 + space + │ + space + col2 + space + │
	// That's 7 chars for borders/spacing, leaving 73 for content
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: false, WidthMin: 22, WidthMax: 22},
		{Number: 2, AutoMerge: false, WidthMin: 51, WidthMax: 51},
	})

	// Basic Information
	t.AppendRow(table.Row{"DAG Name", dag.Name})
	t.AppendRow(table.Row{"Run ID", dagStatus.DAGRunID})
	t.AppendRow(table.Row{"Process ID", formatPID(dagStatus.PID)})
	t.AppendRow(table.Row{"Status", formatStatus(dagStatus.Status)})

	// Timing Information
	if dagStatus.StartedAt != "" && dagStatus.StartedAt != "-" {
		startedAt, _ := stringutil.ParseTime(dagStatus.StartedAt)
		t.AppendRow(table.Row{"Started At", dagStatus.StartedAt})

		if dagStatus.FinishedAt != "" && dagStatus.FinishedAt != "-" {
			finishedAt, _ := stringutil.ParseTime(dagStatus.FinishedAt)
			if !startedAt.IsZero() && !finishedAt.IsZero() {
				duration := finishedAt.Sub(startedAt)
				t.AppendRow(table.Row{"Duration", stringutil.FormatDuration(duration)})
			}
			t.AppendRow(table.Row{"Finished At", dagStatus.FinishedAt})
		} else if dagStatus.Status == status.Running && !startedAt.IsZero() {
			elapsed := time.Since(startedAt)
			t.AppendRow(table.Row{"Running For", stringutil.FormatDuration(elapsed)})
		}
	}

	// Additional Information
	if dagStatus.AttemptID != "" {
		t.AppendRow(table.Row{"Attempt ID", dagStatus.AttemptID})
	}

	// Error information if available
	errors := dagStatus.Errors()
	if len(errors) > 0 {
		errorText := ""
		for i, err := range errors {
			if i > 0 {
				errorText += "\n"
			}
			errorText += err.Error()
		}
		t.AppendRow(table.Row{"Errors", text.WrapSoft(errorText, 50)})
	}

	// Render the overview table
	fmt.Println(t.Render())

	// Step Summary if available
	if len(dagStatus.Nodes) > 0 {
		fmt.Println()
		displayStepSummary(dagStatus.Nodes)
	}

	// Additional status-specific messages
	fmt.Println()
	switch dagStatus.Status {
	case status.Running:
		fmt.Printf("%s The DAG is currently running. Use 'dagu stop %s' to stop it.\n",
			color.YellowString("→"), dag.Name)
	case status.Error:
		fmt.Printf("%s The DAG failed. Use 'dagu retry --run-id=%s %s' to retry.\n",
			color.RedString("✗"), dagStatus.DAGRunID, dag.Name)
	case status.Success:
		fmt.Printf("%s The DAG completed successfully.\n", color.GreenString("✓"))
	case status.PartialSuccess:
		fmt.Printf("%s The DAG completed with partial success.\n", color.YellowString("⚠"))
	case status.Cancel:
		fmt.Printf("%s The DAG was cancelled.\n", color.YellowString("⚠"))
	case status.Queued:
		fmt.Printf("%s The DAG is queued for execution.\n", color.BlueString("●"))
	case status.None:
		fmt.Printf("%s The DAG has not been started yet.\n", color.New(color.Faint).Sprint("○"))
	}
}

// displayStepSummary shows a summary of all steps in the DAG run
func displayStepSummary(nodes []*models.Node) {
	headerColor := color.New(color.FgCyan, color.Bold)

	// Create a boxed header with 80 character width
	fmt.Println(strings.Repeat("─", 80))
	title := "Step Summary"
	padding := (80 - len(title)) / 2
	fmt.Printf("%s%s%s\n",
		strings.Repeat(" ", padding),
		headerColor.Sprint(title),
		strings.Repeat(" ", 80-padding-len(title)))
	fmt.Println(strings.Repeat("─", 80))

	// Count steps by status
	statusCounts := make(map[status.NodeStatus]int)
	for _, node := range nodes {
		statusCounts[node.Status]++
	}

	// Create step summary table
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	// Configure columns to ensure total width is 80 chars
	// Table borders: │ + space + col1 + space + │ + space + col2 + space + │ + space + col3 + space + │ + space + col4 + space + │
	// That's 13 chars for borders/spacing, leaving 67 for content
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: false, WidthMin: 30, WidthMax: 30, WidthMaxEnforcer: func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n]
		}},
		{Number: 2, AutoMerge: false, WidthMin: 13, WidthMax: 13},
		{Number: 3, AutoMerge: false, WidthMin: 12, WidthMax: 12},
		{Number: 4, AutoMerge: false, WidthMin: 12, WidthMax: 12},
	})
	t.AppendHeader(table.Row{"Step Name", "Status", "Started", "Duration"})

	// Show first few steps and any failed steps
	shownSteps := 0
	failedSteps := []*models.Node{}

	for _, node := range nodes {
		if node.Status == status.NodeError {
			failedSteps = append(failedSteps, node)
		}

		duration := ""
		startTime := ""

		if node.StartedAt != "" && node.StartedAt != "-" {
			startedAt, _ := stringutil.ParseTime(node.StartedAt)
			if !startedAt.IsZero() {
				startTime = startedAt.Format("15:04:05")
			}

			if node.FinishedAt != "" && node.FinishedAt != "-" {
				finishedAt, _ := stringutil.ParseTime(node.FinishedAt)
				if !startedAt.IsZero() && !finishedAt.IsZero() {
					duration = stringutil.FormatDuration(finishedAt.Sub(startedAt))
				}
			} else if node.Status == status.NodeRunning && !startedAt.IsZero() {
				duration = stringutil.FormatDuration(time.Since(startedAt))
			}
		}

		t.AppendRow(table.Row{
			node.Step.Name,
			formatNodeStatus(node.Status),
			startTime,
			duration,
		})
		shownSteps++
	}

	fmt.Println(t.Render())

	// Show status summary
	fmt.Println()
	fmt.Print("Status Summary: ")
	first := true
	for status, count := range statusCounts {
		if !first {
			fmt.Print(", ")
		}
		fmt.Printf("%s: %d", formatNodeStatus(status), count)
		first = false
	}
	fmt.Println()

	// Show failed steps if any
	if len(failedSteps) > 0 && len(failedSteps) != shownSteps {
		fmt.Println()
		color.Red("Failed Steps:")
		for _, node := range failedSteps {
			fmt.Printf("  • %s", node.Step.Name)
			if node.Error != "" {
				fmt.Printf(" - %s", node.Error)
			}
			fmt.Println()
		}
	}

	// Show detailed step information with log preview
	fmt.Println()

	// Create a boxed header with 80 character width
	fmt.Println(strings.Repeat("─", 80))
	logTitle := "Step Logs Preview"
	logPadding := (80 - len(logTitle)) / 2
	fmt.Printf("%s%s%s\n",
		strings.Repeat(" ", logPadding),
		headerColor.Sprint(logTitle),
		strings.Repeat(" ", 80-logPadding-len(logTitle)))
	fmt.Println(strings.Repeat("─", 80))

	// Create detailed table
	detailTable := table.NewWriter()
	detailTable.SetStyle(table.StyleLight)
	// Configure columns to ensure total width is 80 chars
	// Table borders: │ + space + col1 + space + │ + space + col2 + space + │ + space + col3 + space + │
	// That's 10 chars for borders/spacing, leaving 70 for content
	// Need to add 1 more char, so we'll make it 71 total
	detailTable.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: false, WidthMin: 17, WidthMax: 17, WidthMaxEnforcer: func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n]
		}},
		{Number: 2, AutoMerge: false, WidthMin: 27, WidthMax: 27},
		{Number: 3, AutoMerge: false, WidthMin: 26, WidthMax: 26},
	})
	detailTable.AppendHeader(table.Row{"Step", "Stdout (first line)", "Stderr (first line)"})

	// Show all steps with log preview
	for _, node := range nodes {
		// Read first line of logs
		stdoutPreview := readFirstLine(node.Stdout)
		stderrPreview := readFirstLine(node.Stderr)

		detailTable.AppendRow(table.Row{
			node.Step.Name,
			stdoutPreview,
			stderrPreview,
		})
	}

	fmt.Println(detailTable.Render())
}

// readFirstLine reads the first line of a file and adds ellipsis if there's more content
func readFirstLine(path string) string {
	if path == "" {
		return "-"
	}

	// #nosec G304 - file path is from trusted DAG execution status data
	file, err := os.Open(path)
	if err != nil {
		return "(unable to read)"
	}
	defer func() {
		_ = file.Close() // ignore close error for status display
	}()

	// Read up to 2KB to handle very long lines and detect binary content
	buffer := make([]byte, 2048)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return "(empty)"
	}

	// Check for binary content (null bytes or high percentage of non-printable chars)
	content := buffer[:n]
	if isBinaryContent(content) {
		return "(binary data)"
	}

	// Convert to string - Go handles invalid UTF-8 gracefully with replacement chars
	text := string(content)

	// Find first line or use entire text if no newline
	lines := strings.SplitN(text, "\n", 2)
	firstLine := lines[0]

	// Check if there's more content
	hasMoreLines := len(lines) > 1 && lines[1] != ""
	hasMoreData := n == len(buffer) // buffer was filled, likely more data

	// For very long single lines, be more aggressive with truncation
	maxDisplayLength := 24
	if hasMoreData && !hasMoreLines {
		// Long line without breaks - show less to indicate truncation
		maxDisplayLength = 21
	}

	// Truncate if too long - simple byte truncation is fine for display
	if len(firstLine) > maxDisplayLength {
		return firstLine[:maxDisplayLength-3] + "..."
	}

	// Add ellipsis if there's more content
	if hasMoreLines || hasMoreData {
		if len(firstLine) > 21 {
			return firstLine[:18] + "..."
		}
		return firstLine + "..."
	}

	return firstLine
}

// isBinaryContent checks if the content appears to be binary data
func isBinaryContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Check for null bytes (strong indicator of binary)
	for _, b := range data {
		if b == 0 {
			return true
		}
	}

	// Count non-printable characters (excluding common whitespace)
	nonPrintable := 0
	for _, b := range data {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider it binary
	return float64(nonPrintable)/float64(len(data)) > 0.3
}

// formatStatus returns a colored status string
func formatStatus(st status.Status) string {
	switch st {
	case status.Success:
		return color.GreenString("Success")
	case status.Error:
		return color.RedString("Failed")
	case status.PartialSuccess:
		return color.YellowString("Partial Success")
	case status.Running:
		return color.New(color.FgHiGreen).Sprint("Running")
	case status.Cancel:
		return color.YellowString("Cancelled")
	case status.Queued:
		return color.BlueString("Queued")
	case status.None:
		return color.New(color.Faint).Sprint("Not Started")
	default:
		return st.String()
	}
}

// formatNodeStatus returns a colored status string for node status
func formatNodeStatus(s status.NodeStatus) string {
	switch s {
	case status.NodeSuccess:
		return color.GreenString("Success")
	case status.NodeError:
		return color.RedString("Failed")
	case status.NodeRunning:
		return color.New(color.FgHiGreen).Sprint("Running")
	case status.NodeCancel:
		return color.YellowString("Cancelled")
	case status.NodeSkipped:
		return color.New(color.Faint).Sprint("Skipped")
	case status.NodePartialSuccess:
		return color.YellowString("Partial Success")
	case status.NodeNone:
		return color.New(color.Faint).Sprint("Not Started")
	default:
		return fmt.Sprintf("%d", s)
	}
}

// formatPID formats the process ID, showing "-" if not available
func formatPID(pid models.PID) string {
	if pid > 0 {
		return fmt.Sprintf("%d", pid)
	}
	return "-"
}

func extractDAGName(ctx *Context, name string) (string, error) {
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		// Read the DAG from the file.
		dagStore, err := ctx.dagStore(nil, nil)
		if err != nil {
			return "", fmt.Errorf("failed to initialize DAG store: %w", err)
		}
		dag, err := dagStore.GetMetadata(ctx, name)
		if err != nil {
			return "", fmt.Errorf("failed to read DAG metadata from file %s: %w", name, err)
		}
		// Return the DAG name.
		return dag.Name, nil
	}

	// Otherwise, treat it as a DAG name.
	return name, nil
}

func extractAttemptID(ctx *Context, name, dagRunID string) (models.DAGRunAttempt, error) {
	if dagRunID != "" {
		// Retrieve the previous run's record for the specified dag-run ID.
		dagRunRef := digraph.NewDAGRunRef(name, dagRunID)
		att, err := ctx.DAGRunStore.FindAttempt(ctx, dagRunRef)
		if err != nil {
			return nil, fmt.Errorf("failed to find run data for dag-run ID %s: %w", dagRunID, err)
		}
		return att, nil
	}

	// If it's not a file, treat it as a DAG name.
	att, err := ctx.DAGRunStore.LatestAttempt(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find the latest run data for DAG %s: %w", name, err)
	}
	return att, nil
}

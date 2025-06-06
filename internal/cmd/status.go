package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/stringutil"
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

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status from attempt: %w", err)
	}

	if status.Status == scheduler.StatusRunning {
		realtimeStatus, err := ctx.DAGRunMgr.GetCurrentStatus(ctx, dag, dagRunID)
		if err != nil {
			return fmt.Errorf("failed to retrieve current status: %w", err)
		}
		if realtimeStatus.DAGRunID == status.DAGRunID {
			status = realtimeStatus
		}
	}

	// Display detailed status information
	displayDetailedStatus(dag, status)

	return nil
}

// displayDetailedStatus renders a formatted table with DAG run information
func displayDetailedStatus(dag *digraph.DAG, status *models.DAGRunStatus) {
	// Create header
	fmt.Println()
	headerColor := color.New(color.FgCyan, color.Bold)
	_, _ = headerColor.Printf("DAG Run Status Report\n")
	fmt.Println(strings.Repeat("=", 60))

	// Create overview table
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: false, WidthMin: 20},
		{Number: 2, AutoMerge: false, WidthMin: 40},
	})

	// Basic Information
	t.AppendRow(table.Row{"DAG Name", dag.Name})
	t.AppendRow(table.Row{"Run ID", status.DAGRunID})
	t.AppendRow(table.Row{"Process ID", formatPID(status.PID)})
	t.AppendRow(table.Row{"Status", formatStatus(status.Status)})
	
	// Timing Information
	if status.StartedAt != "" && status.StartedAt != "-" {
		startedAt, _ := stringutil.ParseTime(status.StartedAt)
		t.AppendRow(table.Row{"Started At", status.StartedAt})
		
		if status.FinishedAt != "" && status.FinishedAt != "-" {
			finishedAt, _ := stringutil.ParseTime(status.FinishedAt)
			if !startedAt.IsZero() && !finishedAt.IsZero() {
				duration := finishedAt.Sub(startedAt)
				t.AppendRow(table.Row{"Duration", formatDuration(duration)})
			}
			t.AppendRow(table.Row{"Finished At", status.FinishedAt})
		} else if status.Status == scheduler.StatusRunning && !startedAt.IsZero() {
			elapsed := time.Since(startedAt)
			t.AppendRow(table.Row{"Running For", formatDuration(elapsed)})
		}
	}

	// Additional Information
	if status.AttemptID != "" {
		t.AppendRow(table.Row{"Attempt ID", status.AttemptID})
	}

	// Error information if available
	errors := status.Errors()
	if len(errors) > 0 {
		errorText := ""
		for i, err := range errors {
			if i > 0 {
				errorText += "\n"
			}
			errorText += err.Error()
		}
		t.AppendRow(table.Row{"Errors", text.WrapSoft(errorText, 40)})
	}

	// Render the overview table
	fmt.Println(t.Render())

	// Step Summary if available
	if len(status.Nodes) > 0 {
		fmt.Println()
		displayStepSummary(status.Nodes)
	}

	// Additional status-specific messages
	fmt.Println()
	switch status.Status {
	case scheduler.StatusRunning:
		fmt.Printf("%s The DAG is currently running. Use 'dagu stop %s' to stop it.\n",
			color.YellowString("→"), dag.Name)
	case scheduler.StatusError:
		fmt.Printf("%s The DAG failed. Use 'dagu retry --run-id=%s %s' to retry.\n",
			color.RedString("✗"), status.DAGRunID, dag.Name)
	case scheduler.StatusSuccess:
		fmt.Printf("%s The DAG completed successfully.\n", color.GreenString("✓"))
	case scheduler.StatusCancel:
		fmt.Printf("%s The DAG was cancelled.\n", color.YellowString("⚠"))
	case scheduler.StatusQueued:
		fmt.Printf("%s The DAG is queued for execution.\n", color.BlueString("●"))
	case scheduler.StatusNone:
		fmt.Printf("%s The DAG has not been started yet.\n", color.New(color.Faint).Sprint("○"))
	}
}

// displayStepSummary shows a summary of all steps in the DAG run
func displayStepSummary(nodes []*models.Node) {
	headerColor := color.New(color.FgCyan, color.Bold)
	_, _ = headerColor.Println("Step Summary")
	fmt.Println(strings.Repeat("-", 60))

	// Count steps by status
	statusCounts := make(map[scheduler.NodeStatus]int)
	for _, node := range nodes {
		statusCounts[node.Status]++
	}

	// Create step summary table
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Step Name", "Status", "Started", "Duration"})

	// Show first few steps and any failed steps
	maxSteps := 10
	shownSteps := 0
	failedSteps := []*models.Node{}

	for _, node := range nodes {
		if node.Status == scheduler.NodeStatusError {
			failedSteps = append(failedSteps, node)
		}

		if shownSteps < maxSteps {
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
						duration = formatDuration(finishedAt.Sub(startedAt))
					}
				} else if node.Status == scheduler.NodeStatusRunning && !startedAt.IsZero() {
					duration = formatDuration(time.Since(startedAt))
				}
			}

			t.AppendRow(table.Row{
				text.WrapSoft(node.Step.Name, 25),
				formatNodeStatus(node.Status),
				startTime,
				duration,
			})
			shownSteps++
		}
	}

	if len(nodes) > maxSteps {
		t.AppendRow(table.Row{
			fmt.Sprintf("... and %d more steps", len(nodes)-maxSteps),
			"",
			"",
			"",
		})
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
}

// formatStatus returns a colored status string
func formatStatus(status scheduler.Status) string {
	switch status {
	case scheduler.StatusSuccess:
		return color.GreenString("Success")
	case scheduler.StatusError:
		return color.RedString("Failed")
	case scheduler.StatusRunning:
		return color.New(color.FgHiGreen).Sprint("Running")
	case scheduler.StatusCancel:
		return color.YellowString("Cancelled")
	case scheduler.StatusQueued:
		return color.BlueString("Queued")
	case scheduler.StatusNone:
		return color.New(color.Faint).Sprint("Not Started")
	default:
		return status.String()
	}
}

// formatNodeStatus returns a colored status string for node status
func formatNodeStatus(status scheduler.NodeStatus) string {
	switch status {
	case scheduler.NodeStatusSuccess:
		return color.GreenString("Success")
	case scheduler.NodeStatusError:
		return color.RedString("Failed")
	case scheduler.NodeStatusRunning:
		return color.New(color.FgHiGreen).Sprint("Running")
	case scheduler.NodeStatusCancel:
		return color.YellowString("Cancelled")
	case scheduler.NodeStatusSkipped:
		return color.New(color.Faint).Sprint("Skipped")
	case scheduler.NodeStatusNone:
		return color.New(color.Faint).Sprint("Not Started")
	default:
		return fmt.Sprintf("%d", status)
	}
}

// formatPID formats the process ID, showing "-" if not available
func formatPID(pid models.PID) string {
	if pid > 0 {
		return fmt.Sprintf("%d", pid)
	}
	return "-"
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
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

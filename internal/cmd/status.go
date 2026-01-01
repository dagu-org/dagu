package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func Status() *cobra.Command {
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

	if dagStatus.Status == core.Running {
		realtimeStatus, err := ctx.DAGRunMgr.GetCurrentStatus(ctx, dag, dagRunID)
		if err != nil {
			return fmt.Errorf("failed to retrieve current status: %w", err)
		}
		if realtimeStatus.DAGRunID == dagStatus.DAGRunID {
			dagStatus = realtimeStatus
		}
	}

	// Display tree-structured status information
	displayTreeStatus(dag, dagStatus)

	return nil
}

// displayTreeStatus renders a tree-structured output with DAG run information
func displayTreeStatus(dag *core.DAG, dagStatus *execution.DAGRunStatus) {
	config := output.DefaultConfig()
	config.ColorEnabled = term.IsTerminal(int(os.Stdout.Fd()))

	renderer := output.NewRenderer(config)
	tree := renderer.RenderDAGStatus(dag, dagStatus)
	fmt.Print(tree)
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

func extractAttemptID(ctx *Context, name, dagRunID string) (execution.DAGRunAttempt, error) {
	if dagRunID != "" {
		// Retrieve the previous run's record for the specified dag-run ID.
		dagRunRef := execution.NewDAGRunRef(name, dagRunID)
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

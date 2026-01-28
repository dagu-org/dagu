package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
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
  --run-id string (optional)     Unique identifier of the DAG-run to check.
                                 If not provided, it will show the status of the
                                 most recent DAG-run for the given name.
  --sub-run-id string (optional) Unique identifier of a sub DAG-run.
                                 Requires --run-id to be provided.
                                 Use this to check the status of nested DAG executions.

Example:
  dagu status --run-id=abc123 my_dag
  dagu status my_dag  # Shows status of the most recent DAG-run
  dagu status --run-id=abc123 --sub-run-id=def456 my_dag  # Shows status of a sub DAG-run
`,
			Args: cobra.ExactArgs(1),
		}, statusFlags, runStatus,
	)
}

var statusFlags = []commandLineFlag{
	dagRunIDFlagStatus,
	subDAGRunIDFlagStatus,
}

func runStatus(ctx *Context, args []string) error {
	dagRunID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to get dag-run ID: %w", err)
	}

	subDAGRunID, err := ctx.StringParam("sub-run-id")
	if err != nil {
		return fmt.Errorf("failed to get sub-dag-run ID: %w", err)
	}

	// Validate: sub-run-id requires run-id
	if subDAGRunID != "" && dagRunID == "" {
		return fmt.Errorf("--sub-run-id requires --run-id to be provided (root DAG run context is needed)")
	}

	name, err := extractDAGName(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}

	// Get the attempt (either root or sub)
	attempt, err := extractAttemptForStatus(ctx, name, dagRunID, subDAGRunID)
	if err != nil {
		return fmt.Errorf("failed to extract attempt: %w", err)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from run data: %w", err)
	}

	dagStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status from attempt: %w", err)
	}

	// For running DAGs, try to get real-time status
	// Note: For sub DAGs, we use the stored status as-is since they may be running
	// on different workers and don't have a direct socket connection
	if dagStatus.Status == core.Running && subDAGRunID == "" {
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
func displayTreeStatus(dag *core.DAG, dagStatus *exec.DAGRunStatus) {
	config := output.DefaultConfig()
	config.ColorEnabled = term.IsTerminal(int(os.Stdout.Fd()))

	renderer := output.NewRenderer(config)
	tree := renderer.RenderDAGStatus(dag, dagStatus)
	fmt.Print(tree)
}

func extractDAGName(ctx *Context, name string) (string, error) {
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		// Read the DAG from the file.
		dagStore, err := ctx.dagStore(dagStoreConfig{})
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

func extractAttemptID(ctx *Context, name, dagRunID string) (exec.DAGRunAttempt, error) {
	if dagRunID != "" {
		// Retrieve the previous run's record for the specified dag-run ID.
		dagRunRef := exec.NewDAGRunRef(name, dagRunID)
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

// extractAttemptForStatus extracts the appropriate DAGRunAttempt based on the provided IDs.
// If subDAGRunID is provided, it finds the sub dag-run under the root run.
// Otherwise, it behaves like extractAttemptID for root dag-runs.
func extractAttemptForStatus(ctx *Context, name, dagRunID, subDAGRunID string) (exec.DAGRunAttempt, error) {
	// If no sub-run-id, use the existing logic for root DAG runs
	if subDAGRunID == "" {
		return extractAttemptID(ctx, name, dagRunID)
	}

	// For sub DAG runs, we need the root run-id (already validated in runStatus)
	dagRunRef := exec.NewDAGRunRef(name, dagRunID)

	// Find the sub DAG run attempt
	attempt, err := ctx.DAGRunStore.FindSubAttempt(ctx, dagRunRef, subDAGRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to find sub dag-run with ID %s under root %s: %w",
			subDAGRunID, dagRunID, err)
	}

	return attempt, nil
}

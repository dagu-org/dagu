package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"
)

func CmdRetry() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "retry [flags] /path/to/spec.yaml",
			Short: "Retry a workflow",
			Long: `Re-execute a previously run DAG using its unique workflow ID.

Example:
  dagu retry my_dag.yaml --workflow-id=abc123

This command is useful for recovering from errors or transient issues by re-running the DAG.
`,
			Args: cobra.ExactArgs(1),
		}, retryFlags, runRetry,
	)
}

var retryFlags = []commandLineFlag{workflowIDFlagRetry}

func runRetry(ctx *Context, args []string) error {
	workflowID, err := ctx.Command.Flags().GetString("workflow-id")
	if err != nil {
		return fmt.Errorf("failed to get workflow ID: %w", err)
	}

	name := args[0]

	// Retrieve the previous run data for specified workflow ID.
	ref := digraph.NewWorkflowRef(name, workflowID)
	runRecord, err := ctx.HistoryRepo.Find(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to find the record for workflow ID %s: %w", workflowID, err)
	}

	// Read the detailed status of the previous status.
	status, err := runRecord.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}

	// Get the DAG instance from the execution history.
	dag, err := runRecord.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from record: %w", err)
	}

	// The retry command is currently only supported for root DAGs.
	if err := executeRetry(ctx, dag, status, status.Workflow()); err != nil {
		logger.Error(ctx, "Failed to execute retry", "path", name, "err", err)
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

func executeRetry(ctx *Context, dag *digraph.DAG, status *models.Status, rootRun digraph.WorkflowRef) error {
	logger.Debug(ctx, "Executing retry", "name", dag.Name, "workflowId", status.WorkflowID)

	// We use the same log file for the retry as the original run.
	logFile, err := fileutil.OpenOrCreateFile(status.Log)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Info(ctx, "DAG retry initiated", "DAG", dag.Name, "workflowId", status.WorkflowID, "logFile", logFile.Name())

	// Update the context with the log file
	ctx.LogToFile(logFile)

	dr, err := ctx.dagRepo(nil, []string{filepath.Dir(dag.Location)})
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	agentInstance := agent.New(
		status.WorkflowID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.HistoryMgr,
		dr,
		ctx.HistoryRepo,
		rootRun,
		agent.Options{
			RetryTarget: status,
			Parent:      status.Parent,
		},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute the workflow %s (workflow ID: %s): %w", dag.Name, status.WorkflowID, err)
		}
	}

	// Print the summary of the execution if the quiet flag is not set.
	if !ctx.Quiet {
		agentInstance.PrintSummary(ctx)
	}

	return nil
}

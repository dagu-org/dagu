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
			Use:   "retry [flags] <workflow name>",
			Short: "Retry a previously executed workflow",
			Long: `Create a new run for a previously executed workflow using the same workflow ID.

Unlike restart, which creates a new workflow with a new ID, retry creates a new run within 
the same workflow ID. This preserves the workflow history and allows for multiple attempts
of the same workflow instance.

Flags:
  --workflow-id string (required) Unique identifier of the workflow to retry.

Example:
  dagu retry --workflow-id=abc123 my_dag

This command is useful for recovering from errors or transient issues by creating a new run
of the same workflow without changing its identity.
`,
			Args: cobra.ExactArgs(1),
		}, retryFlags, runRetry,
	)
}

var retryFlags = []commandLineFlag{workflowIDFlagRetry}

func runRetry(ctx *Context, args []string) error {
	workflowID, _ := ctx.StringParam("workflow-id")
	name := args[0]

	// Retrieve the previous run data for specified workflow ID.
	ref := digraph.NewDAGRunRef(name, workflowID)
	runRecord, err := ctx.HistoryStore.FindAttempt(ctx, ref)
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
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

func executeRetry(ctx *Context, dag *digraph.DAG, status *models.DAGRunStatus, rootRun digraph.DAGRunRef) error {
	logger.Debug(ctx, "Executing workflow retry", "name", dag.Name, "dagRunId", status.RunID)

	// We use the same log file for the retry as the original run.
	logFile, err := fileutil.OpenOrCreateFile(status.Log)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Info(ctx, "Workflow retry initiated", "DAG", dag.Name, "dagRunId", status.RunID, "logFile", logFile.Name())

	// Update the context with the log file
	ctx.LogToFile(logFile)

	dr, err := ctx.dagStore(nil, []string{filepath.Dir(dag.Location)})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	agentInstance := agent.New(
		status.RunID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.HistoryMgr,
		dr,
		ctx.HistoryStore,
		ctx.ProcStore,
		rootRun,
		agent.Options{
			RetryTarget:  status,
			ParentDAGRun: status.Parent,
		},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute the workflow %s (workflow ID: %s): %w", dag.Name, status.RunID, err)
		}
	}

	// Print the summary of the execution if the quiet flag is not set.
	if !ctx.Quiet {
		agentInstance.PrintSummary(ctx)
	}

	return nil
}

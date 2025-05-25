package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"
)

func CmdRestart() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "restart [flags] <DAG definition or workflow name>",
			Short: "Restart a running workflow with a new ID",
			Long: `Stop a currently running workflow and immediately restart it with the same configuration but with a new workflow ID.

This command creates a new workflow instance based on the same DAG definition as the original workflow.
It first gracefully stops the active workflow, ensuring all resources are properly released, then
initiates a new workflow with identical parameters.

Flags:
  --workflow-id string (optional) Unique identifier of the workflow to restart. If not provided,
                                  the command will find the current running workflow by the given DAG name.

Example:
  dagu restart --workflow-id=abc123 my_dag
`,
			Args: cobra.ExactArgs(1),
		}, restartFlags, runRestart,
	)
}

var restartFlags = []commandLineFlag{
	workflowIDFlagRestart,
}

func runRestart(ctx *Context, args []string) error {
	workflowID, err := ctx.StringParam("workflow-id")
	if err != nil {
		return fmt.Errorf("failed to get workflow ID: %w", err)
	}

	name := args[0]

	var run models.DAGRunAttempt
	if workflowID != "" {
		// Retrieve the previous run for the specified workflow ID.
		ref := digraph.NewDAGRunRef(name, workflowID)
		r, err := ctx.HistoryStore.FindAttempt(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to find the run for workflow ID %s: %w", workflowID, err)
		}
		run = r
	} else {
		r, err := ctx.HistoryStore.LatestAttempt(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to find the latest execution history for DAG %s: %w", name, err)
		}
		run = r
	}

	status, err := run.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}
	if status.Status != scheduler.StatusRunning {
		return fmt.Errorf("workflow %s is not running", name)
	}

	dag, err := run.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from execution history: %w", err)
	}

	if err := handleRestartProcess(ctx, dag, workflowID); err != nil {
		return fmt.Errorf("restart process failed for DAG %s: %w", dag.Name, err)
	}

	return nil
}

func handleRestartProcess(ctx *Context, d *digraph.DAG, workflowID string) error {
	// Stop if running
	if err := stopDAGIfRunning(ctx, ctx.HistoryMgr, d, workflowID); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	// Wait before restart if configured
	if d.RestartWait > 0 {
		logger.Info(ctx, "Waiting for restart", "duration", d.RestartWait)
		time.Sleep(d.RestartWait)
	}

	// Execute the exact same DAG with the same parameters but a new workflow ID
	return executeDAG(ctx, ctx.HistoryMgr, d)
}

func executeDAG(ctx *Context, cli history.DAGRunManager, dag *digraph.DAG) error {
	workflowID, err := genWorkflowID()
	if err != nil {
		return fmt.Errorf("failed to generate workflow ID: %w", err)
	}

	logFile, err := ctx.OpenLogFile(dag, workflowID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	ctx.LogToFile(logFile)

	logger.Info(ctx, "Workflow restart initiated", "DAG", dag.Name, "dagRunId", workflowID, "logFile", logFile.Name())

	dr, err := ctx.dagStore(nil, []string{filepath.Dir(dag.Location)})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	agentInstance := agent.New(
		workflowID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dr,
		ctx.HistoryStore,
		ctx.ProcStore,
		digraph.NewDAGRunRef(dag.Name, workflowID),
		agent.Options{Dry: false})

	listenSignals(ctx, agentInstance)
	if err := agentInstance.Run(ctx); err != nil {
		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("workflow failed: %w", err)
		}
	}

	return nil
}

func stopDAGIfRunning(ctx context.Context, cli history.DAGRunManager, dag *digraph.DAG, workflowID string) error {
	status, err := cli.GetCurrentStatus(ctx, dag, workflowID)
	if err != nil {
		return fmt.Errorf("failed to get current status: %w", err)
	}

	if status.Status == scheduler.StatusRunning {
		logger.Infof(ctx, "Stopping: %s", dag.Name)
		if err := stopRunningDAG(ctx, cli, dag, workflowID); err != nil {
			return fmt.Errorf("failed to stop running DAG: %w", err)
		}
	}
	return nil
}

func stopRunningDAG(ctx context.Context, cli history.DAGRunManager, dag *digraph.DAG, workflowID string) error {
	const stopPollInterval = 100 * time.Millisecond
	for {
		status, err := cli.GetCurrentStatus(ctx, dag, workflowID)
		if err != nil {
			return fmt.Errorf("failed to get current status: %w", err)
		}

		if status.Status != scheduler.StatusRunning {
			return nil
		}

		if err := cli.Stop(ctx, dag, workflowID); err != nil {
			return fmt.Errorf("failed to stop DAG: %w", err)
		}

		time.Sleep(stopPollInterval)
	}
}

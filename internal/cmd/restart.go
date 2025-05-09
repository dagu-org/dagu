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
			Use:   "restart --workflow-id=abc123 <DAG name or workflow name>",
			Short: "Restart a running DAG",
			Long: `Stop the currently running DAG and immediately restart it with the same configuration but with a new workflow ID.

Flags:
  --workflow-id string (optional) Unique identifier for tracking the restart execution.

Example:
  dagu restart --workflow-id=abc123 my_dag

This command gracefully stops the active workflow before restarting it.
If the workflow ID is not provided, it will find the current running workflow by the given DAG name.
`,
			Args: cobra.ExactArgs(1),
		}, restartFlags, runRestart,
	)
}

var restartFlags = []commandLineFlag{
	workflowIDFlagRestart,
}

func runRestart(ctx *Context, args []string) error {
	workflowID, err := ctx.Command.Flags().GetString("workflow-id")
	if err != nil {
		return fmt.Errorf("failed to get workflow ID: %w", err)
	}

	name := args[0]

	var run models.Run
	if workflowID != "" {
		// Retrieve the previous run for the specified workflow ID.
		ref := digraph.NewWorkflowRef(name, workflowID)
		r, err := ctx.HistoryRepo.FindRun(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to find the run for workflow ID %s: %w", workflowID, err)
		}
		run = r
	} else {
		r, err := ctx.HistoryRepo.LatestRun(ctx, name)
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

func executeDAG(ctx *Context, cli history.Manager, dag *digraph.DAG) error {
	workflowID, err := getWorkflowID()
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

	logger.Info(ctx, "workflow restart initiated", "DAG", dag.Name, "workflowId", workflowID, "logFile", logFile.Name())

	dr, err := ctx.dagRepo(nil, []string{filepath.Dir(dag.Location)})
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
		ctx.HistoryRepo,
		digraph.NewWorkflowRef(dag.Name, workflowID),
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

func stopDAGIfRunning(ctx context.Context, cli history.Manager, dag *digraph.DAG, workflowID string) error {
	status, err := cli.GetDAGRealtimeStatus(ctx, dag, workflowID)
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

func stopRunningDAG(ctx context.Context, cli history.Manager, dag *digraph.DAG, workflowID string) error {
	const stopPollInterval = 100 * time.Millisecond
	for {
		status, err := cli.GetDAGRealtimeStatus(ctx, dag, workflowID)
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

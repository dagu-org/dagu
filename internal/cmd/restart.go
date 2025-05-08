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
			Use:   "restart --exec-id=abc123 dagName",
			Short: "Restart a running DAG",
			Long: `Stop the currently running DAG and immediately restart it with the same configuration but with a new execution ID.

Flags:
  --exec-id string (optional) Unique identifier for tracking the restart execution.

Example:
  dagu restart --exec-id=abc123 dagName

This command gracefully stops the active DAG run before restarting it.
If the execution ID is not provided, it will find the current running DAG by name.
`,
			Args: cobra.ExactArgs(1),
		}, restartFlags, runRestart,
	)
}

var restartFlags = []commandLineFlag{
	execIDFlagRestart,
}

func runRestart(ctx *Context, args []string) error {
	reqID, err := ctx.Command.Flags().GetString("exec-id")
	if err != nil {
		return fmt.Errorf("failed to get execution ID: %w", err)
	}

	name := args[0]

	var record models.Record
	if reqID != "" {
		// Retrieve the previous run's record for the specified execution ID.
		ref := digraph.NewExecRef(name, reqID)
		r, err := ctx.HistoryRepo.Find(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to find the record for execution ID %s: %w", reqID, err)
		}
		record = r
	} else {
		r, err := ctx.HistoryRepo.Latest(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to find the latest run record for DAG %s: %w", name, err)
		}
		record = r
	}

	status, err := record.ReadStatus(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read status", "err", err)
		return fmt.Errorf("failed to read status: %w", err)
	}
	if status.Status != scheduler.StatusRunning {
		logger.Error(ctx, "DAG is not running", "name", name)
	}

	dag, err := record.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG from run record", "err", err)
		return fmt.Errorf("failed to read DAG from run record: %w", err)
	}

	if err := handleRestartProcess(ctx, dag, reqID); err != nil {
		logger.Error(ctx, "Failed to restart DAG", "name", dag.Name, "err", err)
		return fmt.Errorf("restart process failed for DAG %s: %w", dag.Name, err)
	}

	return nil
}

func handleRestartProcess(ctx *Context, d *digraph.DAG, reqID string) error {
	// Stop if running
	if err := stopDAGIfRunning(ctx, ctx.HistoryMgr, d, reqID); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	// Wait before restart if configured
	if d.RestartWait > 0 {
		logger.Info(ctx, "Waiting for restart", "duration", d.RestartWait)
		time.Sleep(d.RestartWait)
	}

	// Execute the exact same DAG with the same parameters but a new execution ID
	return executeDAG(ctx, ctx.HistoryMgr, d)
}

func executeDAG(ctx *Context, cli history.Manager, dag *digraph.DAG) error {
	reqID, err := genReqID()
	if err != nil {
		return fmt.Errorf("failed to generate execution ID: %w", err)
	}

	logFile, err := ctx.OpenLogFile(dag, reqID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	ctx.LogToFile(logFile)

	logger.Info(ctx, "DAG restart initiated", "DAG", dag.Name, "execId", reqID, "logFile", logFile.Name())

	dr, err := ctx.dagRepo(nil, []string{filepath.Dir(dag.Location)})
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	agentInstance := agent.New(
		reqID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dr,
		ctx.HistoryRepo,
		digraph.NewExecRef(dag.Name, reqID),
		agent.Options{Dry: false})

	listenSignals(ctx, agentInstance)
	if err := agentInstance.Run(ctx); err != nil {
		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("DAG run failed: %w", err)
		}
	}

	return nil
}

func stopDAGIfRunning(ctx context.Context, cli history.Manager, dag *digraph.DAG, requestID string) error {
	status, err := cli.GetRealtimeStatus(ctx, dag, requestID)
	if err != nil {
		return fmt.Errorf("failed to get current status: %w", err)
	}

	if status.Status == scheduler.StatusRunning {
		logger.Infof(ctx, "Stopping: %s", dag.Name)
		if err := stopRunningDAG(ctx, cli, dag, requestID); err != nil {
			return fmt.Errorf("failed to stop running DAG: %w", err)
		}
	}
	return nil
}

func stopRunningDAG(ctx context.Context, cli history.Manager, dag *digraph.DAG, requestID string) error {
	const stopPollInterval = 100 * time.Millisecond
	for {
		status, err := cli.GetRealtimeStatus(ctx, dag, requestID)
		if err != nil {
			return fmt.Errorf("failed to get current status: %w", err)
		}

		if status.Status != scheduler.StatusRunning {
			return nil
		}

		if err := cli.Stop(ctx, dag, requestID); err != nil {
			return fmt.Errorf("failed to stop DAG: %w", err)
		}

		time.Sleep(stopPollInterval)
	}
}

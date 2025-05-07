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
	"github.com/spf13/cobra"
)

func CmdRestart() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "restart --request-id=abc123 dagName",
			Short: "Restart a running DAG",
			Long: `Stop the currently running DAG and immediately restart it with the same configuration but with a new request ID.

Flags:
  --request-id string (optional) Unique identifier for tracking the restart execution.

Example:
  dagu restart --request-id=abc123 dagName

This command gracefully stops the active DAG run before restarting it.
If the request ID is not provided, it will find the current running DAG by name.
`,
			Args: cobra.ExactArgs(1),
		}, restartFlags, runRestart,
	)
}

var restartFlags = []commandLineFlag{
	requestIDFlagRestart,
}

func runRestart(ctx *Context, args []string) error {
	requestID, err := ctx.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	dagName := args[0]

	var record history.Record
	if requestID != "" {
		// Retrieve the previous run's runstore record for the specified request ID.
		r, err := ctx.runStore().Find(ctx, dagName, requestID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve historical run", "requestID", requestID, "err", err)
			return fmt.Errorf("failed to retrieve historical run for request ID %s: %w", requestID, err)
		}
		record = r
	} else {
		r, err := ctx.runStore().Latest(ctx, dagName)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve latest runstore record", "dagName", dagName, "err", err)
			return fmt.Errorf("failed to retrieve latest runstore record for DAG %s: %w", dagName, err)
		}
		record = r
	}

	status, err := record.ReadStatus(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read status", "err", err)
		return fmt.Errorf("failed to read status: %w", err)
	}
	if status.Status != scheduler.StatusRunning {
		logger.Error(ctx, "DAG is not running", "dagName", dagName)
	}

	dag, err := record.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG from runstore record", "err", err)
		return fmt.Errorf("failed to read DAG from runstore record: %w", err)
	}

	if err := handleRestartProcess(ctx, dag, requestID); err != nil {
		logger.Error(ctx, "Failed to restart DAG", "dagName", dag.Name, "err", err)
		return fmt.Errorf("restart process failed for DAG %s: %w", dag.Name, err)
	}

	return nil
}

func handleRestartProcess(ctx *Context, dag *digraph.DAG, requestID string) error {
	cli, err := ctx.HistoryManager()
	if err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	// Stop if running
	if err := stopDAGIfRunning(ctx, cli, dag, requestID); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	// Wait before restart if configured
	if dag.RestartWait > 0 {
		logger.Info(ctx, "Waiting for restart", "duration", dag.RestartWait)
		time.Sleep(dag.RestartWait)
	}

	// Execute the exact same DAG with the same parameters but a new request ID
	return executeDAG(ctx, cli, dag)
}

func executeDAG(ctx *Context, cli history.Manager, dag *digraph.DAG) error {
	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	logFile, err := ctx.OpenLogFile(dag, requestID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	ctx.LogToFile(logFile)

	logger.Info(ctx, "DAG restart initiated", "DAG", dag.Name, "requestID", requestID, "logFile", logFile.Name())

	dagStore, err := ctx.dagStore([]string{filepath.Dir(dag.Location)})
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	rootDAG := digraph.NewRootDAG(dag.Name, requestID)

	agentInstance := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		ctx.runStore(),
		rootDAG,
		agent.Options{Dry: false})

	listenSignals(ctx, agentInstance)
	if err := agentInstance.Run(ctx); err != nil {
		if ctx.quiet {
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

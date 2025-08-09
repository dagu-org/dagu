package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"
)

func CmdRestart() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "restart [flags] <DAG name>",
			Short: "Restart a running DAG-run with a new ID",
			Long: `Stop a currently running DAG-run and immediately restart it with the same configuration but with a new DAG-run ID.

It first gracefully stops the active DAG-run, ensuring all resources are properly released, then
initiates a new DAG-run with identical parameters.

Flags:
  --run-id string (optional) Unique identifier of the DAG-run to restart. If not provided,
                             the command will find the current running DAG-run by the given DAG name.

Example:
  dagu restart --run-id=abc123 my_dag
`,
			Args: cobra.ExactArgs(1),
		}, restartFlags, runRestart,
	)
}

var restartFlags = []commandLineFlag{
	dagRunIDFlagRestart,
}

func runRestart(ctx *Context, args []string) error {
	dagRunID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to get dag-run ID: %w", err)
	}

	name := args[0]

	var attempt models.DAGRunAttempt
	if dagRunID != "" {
		// Retrieve the previous run for the specified dag-run ID.
		dagRunRef := digraph.NewDAGRunRef(name, dagRunID)
		att, err := ctx.DAGRunStore.FindAttempt(ctx, dagRunRef)
		if err != nil {
			return fmt.Errorf("failed to find the run for dag-run ID %s: %w", dagRunID, err)
		}
		attempt = att
	} else {
		att, err := ctx.DAGRunStore.LatestAttempt(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to find the latest execution history for DAG %s: %w", name, err)
		}
		attempt = att
	}

	dagStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}
	if dagStatus.Status != status.Running {
		return fmt.Errorf("DAG %s is not running", name)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from execution history: %w", err)
	}

	if err := handleRestartProcess(ctx, dag, dagRunID); err != nil {
		return fmt.Errorf("restart process failed for DAG %s: %w", dag.Name, err)
	}

	return nil
}

func handleRestartProcess(ctx *Context, d *digraph.DAG, dagRunID string) error {
	// Stop if running
	if err := stopDAGIfRunning(ctx, ctx.DAGRunMgr, d, dagRunID); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	// Wait before restart if configured
	if d.RestartWait > 0 {
		logger.Info(ctx, "Waiting for restart", "duration", d.RestartWait)
		time.Sleep(d.RestartWait)
	}

	// Execute the exact same DAG with the same parameters but a new dag-run ID
	return executeDAG(ctx, ctx.DAGRunMgr, d)
}

func executeDAG(ctx *Context, cli dagrun.Manager, dag *digraph.DAG) error {
	dagRunID, err := genRunID()
	if err != nil {
		return fmt.Errorf("failed to generate dag-run ID: %w", err)
	}

	logFile, err := ctx.OpenLogFile(dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	ctx.LogToFile(logFile)

	logger.Info(ctx, "dag-run restart initiated", "DAG", dag.Name, "dagRunId", dagRunID, "logFile", logFile.Name())

	dr, err := ctx.dagStore(nil, []string{filepath.Dir(dag.Location)})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	agentInstance := agent.New(
		dagRunID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dr,
		ctx.DAGRunStore,
		ctx.ProcStore,
		ctx.ServiceRegistry,
		digraph.NewDAGRunRef(dag.Name, dagRunID),
		agent.Options{Dry: false})

	listenSignals(ctx, agentInstance)
	if err := agentInstance.Run(ctx); err != nil {
		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("dag-run failed: %w", err)
		}
	}

	return nil
}

func stopDAGIfRunning(ctx context.Context, cli dagrun.Manager, dag *digraph.DAG, dagRunID string) error {
	dagStatus, err := cli.GetCurrentStatus(ctx, dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to get current status: %w", err)
	}

	if dagStatus.Status == status.Running {
		logger.Infof(ctx, "Stopping: %s", dag.Name)
		if err := stopRunningDAG(ctx, cli, dag, dagRunID); err != nil {
			return fmt.Errorf("failed to stop running DAG: %w", err)
		}
	}
	return nil
}

func stopRunningDAG(ctx context.Context, cli dagrun.Manager, dag *digraph.DAG, dagRunID string) error {
	const stopPollInterval = 100 * time.Millisecond
	for {
		dagStatus, err := cli.GetCurrentStatus(ctx, dag, dagRunID)
		if err != nil {
			return fmt.Errorf("failed to get current status: %w", err)
		}

		if dagStatus.Status != status.Running {
			return nil
		}

		if err := cli.Stop(ctx, dag, dagRunID); err != nil {
			return fmt.Errorf("failed to stop DAG: %w", err)
		}

		time.Sleep(stopPollInterval)
	}
}

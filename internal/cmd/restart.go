package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/spf13/cobra"
)

func CmdRestart() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "restart [flags] /path/to/spec.yaml",
			Short: "Restart a running DAG",
			Long: `Stop the currently running DAG and immediately restart it with the same configuration.

Flags:
  --request-id string   (Optional) Unique identifier for tracking the restart execution.

Example:
  dagu restart my_dag.yaml --request-id=abc123

This command gracefully stops the active DAG run before reinitiating it.
`,
			Args: cobra.ExactArgs(1),
		}, restartFlags, runRestart,
	)
}

var restartFlags = []commandLineFlag{}

func runRestart(ctx *Context, args []string) error {
	specFilePath := args[0]

	dag, err := digraph.Load(ctx, specFilePath, digraph.WithBaseConfig(ctx.cfg.Paths.BaseConfig))
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "path", specFilePath, "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", specFilePath, err)
	}

	if err := handleRestartProcess(ctx, dag, specFilePath); err != nil {
		logger.Error(ctx, "Failed to restart process", "path", specFilePath, "err", err)
		return fmt.Errorf("restart process failed for DAG %s: %w", dag.Name, err)
	}

	return nil
}

func handleRestartProcess(ctx *Context, dag *digraph.DAG, specFilePath string) error {
	cli, err := ctx.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	// Stop if running
	if err := stopDAGIfRunning(ctx, cli, dag); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	// Wait before restart if configured
	waitForRestart(ctx, dag.RestartWait)

	// Get previous parameters
	status, err := getPreviousRunStatus(ctx, cli, dag)
	if err != nil {
		return fmt.Errorf("failed to get previous run parameters: %w", err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(ctx.cfg.Paths.BaseConfig),
	}
	if status.Params != "" {
		// backward compatibility
		loadOpts = append(loadOpts, digraph.WithParams(status.Params))
	} else {
		loadOpts = append(loadOpts, digraph.WithParams(status.ParamsList))
	}

	// Reload DAG with parameters
	dag, err = digraph.Load(ctx, specFilePath, loadOpts...)
	if err != nil {
		return fmt.Errorf("failed to reload DAG with params: %w", err)
	}

	return executeDAG(ctx, cli, dag)
}

func executeDAG(ctx *Context, cli client.Client, dag *digraph.DAG) error {
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

	dagStore, err := ctx.dagStore()
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
		ctx.historyStore(),
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

func stopDAGIfRunning(ctx context.Context, cli client.Client, dag *digraph.DAG) error {
	status, err := cli.GetCurrentStatus(ctx, dag)
	if err != nil {
		return fmt.Errorf("failed to get current status: %w", err)
	}

	if status.Status == scheduler.StatusRunning {
		logger.Infof(ctx, "Stopping: %s", dag.Name)
		if err := stopRunningDAG(ctx, cli, dag); err != nil {
			return fmt.Errorf("failed to stop running DAG: %w", err)
		}
	}
	return nil
}

func stopRunningDAG(ctx context.Context, cli client.Client, dag *digraph.DAG) error {
	const stopPollInterval = 100 * time.Millisecond
	for {
		status, err := cli.GetCurrentStatus(ctx, dag)
		if err != nil {
			return fmt.Errorf("failed to get current status: %w", err)
		}

		if status.Status != scheduler.StatusRunning {
			return nil
		}

		if err := cli.StopDAG(ctx, dag); err != nil {
			return fmt.Errorf("failed to stop DAG: %w", err)
		}

		time.Sleep(stopPollInterval)
	}
}

func waitForRestart(ctx context.Context, restartWait time.Duration) {
	if restartWait > 0 {
		logger.Info(ctx, "Waiting for restart", "duration", restartWait)
		time.Sleep(restartWait)
	}
}

func getPreviousRunStatus(ctx context.Context, cli client.Client, dag *digraph.DAG) (persistence.Status, error) {
	status, err := cli.GetLatestStatus(ctx, dag)
	if err != nil {
		return persistence.Status{}, fmt.Errorf("failed to get latest status: %w", err)
	}
	return status, nil
}

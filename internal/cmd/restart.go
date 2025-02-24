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
	"github.com/dagu-org/dagu/internal/persistence/model"
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

This command gracefully stops the active DAG execution before reinitiating it.
`,
			Args: cobra.ExactArgs(1),
		}, restartFlags, runRestart,
	)
}

var restartFlags = []commandLineFlag{}

func runRestart(cmd *Command, args []string) error {
	ctx := cmd.ctx
	specFilePath := args[0]

	dag, err := digraph.Load(ctx, specFilePath, digraph.WithBaseConfig(cmd.cfg.Paths.BaseConfig))
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "path", specFilePath, "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", specFilePath, err)
	}

	if err := handleRestartProcess(cmd, dag, specFilePath); err != nil {
		logger.Error(ctx, "Failed to restart process", "path", specFilePath, "err", err)
		return fmt.Errorf("restart process failed for DAG %s: %w", dag.Name, err)
	}

	return nil
}

func handleRestartProcess(cmd *Command, dag *digraph.DAG, specFilePath string) error {
	cli, err := cmd.Client()
	if err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	// Stop if running
	if err := stopDAGIfRunning(cmd.ctx, cli, dag); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	// Wait before restart if configured
	waitForRestart(cmd.ctx, dag.RestartWait)

	// Get previous parameters
	status, err := getPreviousExecutionStatus(cmd.ctx, cli, dag)
	if err != nil {
		return fmt.Errorf("failed to get previous execution parameters: %w", err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(cmd.cfg.Paths.BaseConfig),
	}
	if status.Params != "" {
		// backward compatibility
		loadOpts = append(loadOpts, digraph.WithParams(status.Params))
	} else {
		loadOpts = append(loadOpts, digraph.WithParams(status.ParamsList))
	}

	// Reload DAG with parameters
	dag, err = digraph.Load(cmd.ctx, specFilePath, loadOpts...)
	if err != nil {
		return fmt.Errorf("failed to reload DAG with params: %w", err)
	}

	return executeDAG(cmd, cli, dag)
}

func executeDAG(cmd *Command, cli client.Client, dag *digraph.DAG) error {

	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	const logPrefix = "restart_"
	logFile, err := cmd.OpenLogFile(cmd.ctx, logPrefix, dag, requestID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file: %w", err)
	}
	defer logFile.Close()

	ctx := cmd.loggerContextWithFile(logFile)

	logger.Info(ctx, "DAG restart initiated", "DAG", dag.Name, "requestID", requestID, "logFile", logFile.Name())

	dagStore, err := cmd.dagStore()
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	agentInstance := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		cmd.historyStore(),
		agent.Options{Dry: false})

	listenSignals(ctx, agentInstance)
	if err := agentInstance.Run(ctx); err != nil {
		if cmd.quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("DAG execution failed: %w", err)
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

		if err := cli.Stop(ctx, dag); err != nil {
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

func getPreviousExecutionStatus(ctx context.Context, cli client.Client, dag *digraph.DAG) (model.Status, error) {
	status, err := cli.GetLatestStatus(ctx, dag)
	if err != nil {
		return model.Status{}, fmt.Errorf("failed to get latest status: %w", err)
	}
	return status, nil
}

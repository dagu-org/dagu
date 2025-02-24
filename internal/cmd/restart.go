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
	cmd := &cobra.Command{
		Use:   "restart /path/to/spec.yaml",
		Short: "Stop the running DAG and restart it",
		Long:  `dagu restart /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return bindCommonFlags(cmd, nil)
		},
		RunE: wrapRunE(runRestart),
	}
	initCommonFlags(cmd, nil)
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	return cmd
}

func runRestart(cmd *cobra.Command, args []string) error {
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return fmt.Errorf("failed to get quiet flag: %w", err)
	}

	setup, err := createSetup(cmd.Context(), quiet)
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	ctx := setup.ctx
	specFilePath := args[0]

	dag, err := digraph.Load(ctx, specFilePath, digraph.WithBaseConfig(setup.cfg.Paths.BaseConfig))
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "path", specFilePath, "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", specFilePath, err)
	}

	if err := handleRestartProcess(ctx, setup, dag, quiet, specFilePath); err != nil {
		logger.Error(ctx, "Failed to restart process", "path", specFilePath, "err", err)
		return fmt.Errorf("restart process failed for DAG %s: %w", dag.Name, err)
	}

	return nil
}

func handleRestartProcess(ctx context.Context, setup *Setup, dag *digraph.DAG, quiet bool, specFilePath string) error {
	cli, err := setup.Client()
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
	status, err := getPreviousExecutionStatus(ctx, cli, dag)
	if err != nil {
		return fmt.Errorf("failed to get previous execution parameters: %w", err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(setup.cfg.Paths.BaseConfig),
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

	return executeDAG(ctx, cli, setup, dag, quiet)
}

func executeDAG(ctx context.Context, cli client.Client, setup *Setup,
	dag *digraph.DAG, quiet bool) error {

	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	const logPrefix = "restart_"
	logFile, err := setup.OpenLogFile(ctx, logPrefix, dag, requestID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file: %w", err)
	}
	defer logFile.Close()

	ctx = setup.loggerContextWithFile(ctx, quiet, logFile)

	logger.Info(ctx, "DAG restart initiated", "DAG", dag.Name, "requestID", requestID, "logFile", logFile.Name())

	dagStore, err := setup.dagStore()
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
		setup.historyStore(),
		agent.Options{Dry: false})

	listenSignals(ctx, agentInstance)
	if err := agentInstance.Run(ctx); err != nil {
		if quiet {
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

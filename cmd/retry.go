package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/spf13/cobra"
)

const (
	retryPrefix = "retry_"
)

func retryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry --req=<request-id> /path/to/spec.yaml",
		Short: "Retry the DAG execution",
		Long:  `dagu retry --req=<request-id> /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		RunE:  wrapRunE(runRetry),
	}

	cmd.Flags().StringP("req", "r", "", "request-id")
	_ = cmd.MarkFlagRequired("req")
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	return cmd
}

func runRetry(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	setup := newSetup(cfg)

	// Get quiet flag
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return fmt.Errorf("failed to get quiet flag: %w", err)
	}

	requestID, err := cmd.Flags().GetString("req")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	ctx := setup.loggerContext(cmd.Context(), quiet)

	specFilePath := args[0]

	absolutePath, err := filepath.Abs(specFilePath)
	if err != nil {
		logger.Error(ctx, "Failed to resolve absolute path", "path", specFilePath, "err", err)
		return fmt.Errorf("failed to resolve absolute path for %s: %w", specFilePath, err)
	}

	status, err := setup.historyStore().FindByRequestID(ctx, absolutePath, requestID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve historical execution", "requestID", requestID, "err", err)
		return fmt.Errorf("failed to retrieve historical execution for request ID %s: %w", requestID, err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(cfg.Paths.BaseConfig),
	}

	if status.Status.Params != "" {
		// backward compatibility
		loadOpts = append(loadOpts, digraph.WithParams(status.Status.Params))
	} else {
		loadOpts = append(loadOpts, digraph.WithParams(status.Status.ParamsList))
	}

	dag, err := digraph.Load(ctx, absolutePath, loadOpts...)
	if err != nil {
		logger.Error(ctx, "Failed to load DAG specification", "path", specFilePath, "err", err)
		// nolint : staticcheck
		return fmt.Errorf("failed to load DAG specification from %s with params %s: %w",
			specFilePath, status.Status.Params, err)
	}

	// Execute DAG retry
	if err := executeRetry(ctx, dag, setup, status, quiet); err != nil {
		logger.Error(ctx, "Failed to execute retry", "path", specFilePath, "err", err)
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

func executeRetry(ctx context.Context, dag *digraph.DAG, setup *setup, originalStatus *model.StatusFile, quiet bool) error {
	newRequestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate new request ID: %w", err)
	}

	logFile, err := setup.openLogFile(ctx, retryPrefix, dag, newRequestID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	logger.Info(ctx, "DAG retry initiated", "DAG", dag.Name, "originalRequestID", originalStatus.Status.RequestID, "newRequestID", newRequestID, "logFile", logFile.Name())

	ctx = setup.loggerContextWithFile(ctx, quiet, logFile)

	dagStore, err := setup.dagStore()
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	cli, err := setup.client()
	if err != nil {
		logger.Error(ctx, "Failed to initialize client", "err", err)
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	agt := agent.New(
		newRequestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		setup.historyStore(),
		agent.Options{RetryTarget: &originalStatus.Status},
	)

	listenSignals(ctx, agt)

	if err := agt.Run(ctx); err != nil {
		if quiet {
			os.Exit(1)
		} else {
			agt.PrintSummary(ctx)
			return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, newRequestID, err)
		}
	}

	if !quiet {
		agt.PrintSummary(ctx)
	}

	return nil
}

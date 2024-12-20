// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

const (
	restartPrefix    = "restart_"
	stopPollInterval = 100 * time.Millisecond
)

func restartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart /path/to/spec.yaml",
		Short: "Stop the running DAG and restart it",
		Long:  `dagu restart /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		RunE:  runRestart,
	}
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	return cmd
}

func runRestart(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return fmt.Errorf("failed to get quiet flag: %w", err)
	}

	ctx := cmd.Context()
	ctx = logger.WithLogger(ctx, buildLogger(cfg))

	specFilePath := args[0]

	// Load initial DAG configuration
	dag, err := digraph.Load(ctx, cfg.BaseConfig, specFilePath, "")
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", specFilePath, err)
	}

	dataStore := newDataStores(cfg)
	cli := newClient(cfg, dataStore)

	// Handle the restart process
	if err := handleRestartProcess(ctx, cli, cfg, dag, quiet, specFilePath); err != nil {
		return fmt.Errorf("restart process failed for DAG %s: %w", dag.Name, err)
	}

	return nil
}

func handleRestartProcess(ctx context.Context, cli client.Client, cfg *config.Config,
	dag *digraph.DAG, quiet bool, specFilePath string) error {

	// Stop if running
	if err := stopDAGIfRunning(ctx, cli, dag); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	// Wait before restart if configured
	waitForRestart(ctx, dag.RestartWait)

	// Get previous parameters
	params, err := getPreviousExecutionParams(ctx, cli, dag)
	if err != nil {
		return fmt.Errorf("failed to get previous execution parameters: %w", err)
	}

	// Reload DAG with parameters
	dag, err = digraph.Load(ctx, cfg.BaseConfig, specFilePath, params)
	if err != nil {
		return fmt.Errorf("failed to reload DAG with params: %w", err)
	}

	return executeDAG(ctx, cli, cfg, dag, quiet)
}

func executeDAG(ctx context.Context, cli client.Client, cfg *config.Config,
	dag *digraph.DAG, quiet bool) error {

	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	logFile, err := openLogFile(logFileSettings{
		Prefix:    restartPrefix,
		LogDir:    cfg.LogDir,
		DAGLogDir: dag.LogDir,
		DAGName:   dag.Name,
		RequestID: requestID,
	})
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	logger.Info(ctx, "DAG restart initiated",
		"DAG", dag.Name,
		"requestID", requestID,
		"logFile", logFile.Name())

	ctx = logger.WithLogger(ctx, buildLoggerWithFile(logFile, quiet))
	agt := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		newDataStores(cfg),
		&agent.Options{Dry: false})

	listenSignals(ctx, agt)
	if err := agt.Run(ctx); err != nil {
		return fmt.Errorf("DAG execution failed: %w", err)
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

func getPreviousExecutionParams(ctx context.Context, cli client.Client, dag *digraph.DAG) (string, error) {
	status, err := cli.GetLatestStatus(ctx, dag)
	if err != nil {
		return "", fmt.Errorf("failed to get latest status: %w", err)
	}
	return status.Params, nil
}

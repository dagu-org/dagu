// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
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
		RunE:  runRetry,
	}

	cmd.Flags().StringP("req", "r", "", "request-id")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}

func runRetry(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	requestID, err := cmd.Flags().GetString("req")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	ctx := cmd.Context()
	ctx = logger.WithLogger(ctx, buildLogger(cfg, false))

	specFilePath := args[0]

	// Setup execution context
	executionCtx, err := prepareExecutionContext(ctx, cfg, specFilePath, requestID)
	if err != nil {
		return fmt.Errorf("failed to prepare execution context: %w", err)
	}

	// Execute DAG retry
	if err := executeRetry(ctx, executionCtx, cfg); err != nil {
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

type executionContext struct {
	dag           *digraph.DAG
	dataStore     persistence.DataStores
	originalState *model.StatusFile
	absolutePath  string
}

func prepareExecutionContext(ctx context.Context, cfg *config.Config, specFilePath, requestID string) (*executionContext, error) {
	absolutePath, err := filepath.Abs(specFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", specFilePath, err)
	}

	dataStore := newDataStores(cfg)
	historyStore := dataStore.HistoryStore()

	status, err := historyStore.FindByRequestID(ctx, absolutePath, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve historical execution for request ID %s: %w", requestID, err)
	}

	dag, err := digraph.Load(ctx, cfg.BaseConfig, absolutePath, status.Status.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to load DAG specification from %s with params %s: %w",
			specFilePath, status.Status.Params, err)
	}

	return &executionContext{
		dag:           dag,
		dataStore:     dataStore,
		originalState: status,
		absolutePath:  absolutePath,
	}, nil
}

func executeRetry(ctx context.Context, execCtx *executionContext, cfg *config.Config) error {
	newRequestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate new request ID: %w", err)
	}

	logFile, err := openLogFile(logFileSettings{
		Prefix:    retryPrefix,
		LogDir:    cfg.LogDir,
		DAGLogDir: execCtx.dag.LogDir,
		DAGName:   execCtx.dag.Name,
		RequestID: newRequestID,
	})
	if err != nil {
		return fmt.Errorf("failed to create log file for DAG %s: %w", execCtx.dag.Name, err)
	}
	defer logFile.Close()

	cli := newClient(cfg, execCtx.dataStore)

	logger.Info(ctx, "DAG retry initiated",
		"DAG", execCtx.dag.Name,
		"originalRequestID", execCtx.originalState.Status.RequestID,
		"newRequestID", newRequestID,
		"logFile", logFile.Name())

	ctx = logger.WithLogger(ctx, buildLoggerWithFile(cfg, false, logFile))

	agt := agent.New(
		newRequestID,
		execCtx.dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		execCtx.dataStore,
		&agent.Options{RetryTarget: execCtx.originalState.Status},
	)

	listenSignals(ctx, agt)

	if err := agt.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute DAG %s: %w", execCtx.dag.Name, err)
	}

	return nil
}

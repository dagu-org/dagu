// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

const (
	dryPrefix = "dry_"
)

func dryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dry [flags] /path/to/spec.yaml",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry [--params="param1 param2"] /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		RunE:  runDry,
	}
}

func runDry(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cmd.Flags().StringP("params", "p", "", "parameters")
	params, err := cmd.Flags().GetString("params")
	if err != nil {
		return fmt.Errorf("failed to get parameters: %w", err)
	}

	ctx := cmd.Context()
	dag, err := digraph.Load(ctx, cfg.BaseConfig, args[0], removeQuotes(params))
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	logSettings := logFileSettings{
		Prefix:    dryPrefix,
		LogDir:    cfg.LogDir,
		DAGLogDir: dag.LogDir,
		DAGName:   dag.Name,
		RequestID: requestID,
	}

	logFile, err := openLogFile(logSettings)
	if err != nil {
		return fmt.Errorf("failed to create log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	ctx = logger.WithLogger(ctx, buildLoggerWithFile(cfg, false, logFile))
	dataStore := newDataStores(cfg)
	cli := newClient(cfg, dataStore)

	agt := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dataStore,
		&agent.Options{Dry: true},
	)

	listenSignals(ctx, agt)

	if err := agt.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, requestID, err)
	}

	return nil
}
